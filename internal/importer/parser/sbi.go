package parser

import (
	"encoding/csv"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
	"time"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
	"github.com/xuri/excelize/v2"
)

// SBIV1 parses SBI savings account password-protected XLSX statements (format v1).
// Header at row 18: Date, Details, Ref No/Cheque No, Debit, Credit, Balance
// The file is AES-encrypted; a password is required.
type SBIV1 struct{}

func (p *SBIV1) Bank() string          { return "sbi" }
func (p *SBIV1) FormatVersion() string { return "v1" }

func (p *SBIV1) CanParse(header []string) bool {
	has := func(s string) bool {
		for _, h := range header {
			if strings.Contains(h, s) {
				return true
			}
		}
		return false
	}
	return has("details") && has("debit") && has("credit") && has("balance")
}

// ParseXLSX reads an SBI XLSX file using the given decryption password.
func (p *SBIV1) ParseXLSX(path, password string) (ParseResult, error) {
	f, err := excelize.OpenFile(path, excelize.Options{Password: password})
	if err != nil {
		return ParseResult{}, fmt.Errorf("sbi open xlsx: %w", err)
	}
	defer f.Close()

	sheet := f.GetSheetName(0)
	rows, err := f.GetRows(sheet)
	if err != nil {
		return ParseResult{}, fmt.Errorf("sbi get rows: %w", err)
	}

	meta := extractSBIMeta(rows)
	txns, err := parseSBIRows(rows)
	if err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Meta: meta, Transactions: txns}, nil
}

// Parse implements Parser by treating r as a plain CSV (used in tests with decrypted data).
func (p *SBIV1) Parse(r io.Reader) (ParseResult, error) {
	rdr := csv.NewReader(r)
	rdr.LazyQuotes = true
	rdr.FieldsPerRecord = -1

	var records [][]string
	for {
		rec, err := rdr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParseResult{}, fmt.Errorf("sbi csv: %w", err)
		}
		records = append(records, rec)
	}
	txns, err := parseSBIRows(records)
	if err != nil {
		return ParseResult{}, err
	}
	return ParseResult{Transactions: txns}, nil
}

// extractSBIMeta reads account-level info from the pre-header metadata rows.
// SBI XLSX layout (0-indexed rows):
//   row 1 col A: "Mr. Pushkar  Anand\n..." — account holder name (first line only)
//   row 8 col B: "Account Number  :  40393193313"
//   row 10 col B: "IFSC Code  :  SBIN0001222"
func extractSBIMeta(rows [][]string) StatementMeta {
	var meta StatementMeta
	extract := func(cell, prefix string) string {
		idx := strings.Index(strings.ToUpper(cell), strings.ToUpper(prefix))
		if idx < 0 {
			return ""
		}
		val := strings.TrimSpace(cell[idx+len(prefix):])
		return strings.Fields(val)[0]
	}
	for i, row := range rows {
		if i == 0 {
			continue
		}
		colA := safeCol(row, 0)
		colB := safeCol(row, 1)

		if i == 1 && meta.AccountHolder == "" && colA != "" {
			// First line of the cell may contain "\n".
			meta.AccountHolder = strings.TrimSpace(strings.SplitN(colA, "\n", 2)[0])
			meta.AccountHolder = strings.TrimPrefix(meta.AccountHolder, "Mr. ")
			meta.AccountHolder = strings.TrimPrefix(meta.AccountHolder, "Mrs. ")
			meta.AccountHolder = strings.TrimPrefix(meta.AccountHolder, "Ms. ")
		}
		if v := extract(colB, "Account Number  :  "); v != "" && meta.AccountNumber == "" {
			meta.AccountNumber = v
		}
		if v := extract(colB, "IFSC Code  :  "); v != "" && meta.IFSC == "" {
			meta.IFSC = v
		}
		// Stop scanning once we've passed the metadata section.
		if i > 16 {
			break
		}
	}
	return meta
}

func parseSBIRows(rows [][]string) ([]RawTransaction, error) {
	var (
		headerFound bool
		colDate     int
		colDesc     int
		colDebit    int
		colCredit   int
		colBal      int
		colRef      int
	)

	var out []RawTransaction

	for _, row := range rows {
		if !headerFound {
			norm := normalise(row)
			if findCol(norm, "date") >= 0 && findCol(norm, "debit") >= 0 && findCol(norm, "credit") >= 0 {
				headerFound = true
				colDate = findCol(norm, "date")
				colDesc = findCol(norm, "details")
				colRef = findCol(norm, "ref no")
				colDebit = findCol(norm, "debit")
				colCredit = findCol(norm, "credit")
				colBal = findCol(norm, "balance")
			}
			continue
		}

		if len(row) == 0 {
			continue
		}

		rawDate := strings.TrimSpace(safeCol(row, colDate))
		if rawDate == "" {
			continue
		}
		date, err := time.Parse("02/01/2006", rawDate)
		if err != nil {
			break // reached footer
		}

		// Descriptions may contain embedded newlines from Excel cells.
		desc := strings.ReplaceAll(strings.TrimSpace(safeCol(row, colDesc)), "\n", " ")
		ref := strings.TrimSpace(safeCol(row, colRef))

		debitStr := cleanAmount(safeCol(row, colDebit))
		creditStr := cleanAmount(safeCol(row, colCredit))

		var amount int64
		var dir sqlcgen.TxnDirectionEnum

		switch {
		case debitStr != "" && debitStr != "0" && debitStr != "0.00":
			v, err := strconv.ParseFloat(debitStr, 64)
			if err != nil || v == 0 {
				continue
			}
			amount = int64(math.Round(v * 100))
			dir = sqlcgen.TxnDirectionEnumDebit
		case creditStr != "" && creditStr != "0" && creditStr != "0.00":
			v, err := strconv.ParseFloat(creditStr, 64)
			if err != nil || v == 0 {
				continue
			}
			amount = int64(math.Round(v * 100))
			dir = sqlcgen.TxnDirectionEnumCredit
		default:
			continue
		}

		tx := RawTransaction{
			Date:        date,
			Description: desc,
			Amount:      amount,
			Direction:   dir,
			Reference:   ref,
		}

		if colBal >= 0 {
			balStr := cleanAmount(safeCol(row, colBal))
			if balStr != "" {
				if v, err := strconv.ParseFloat(balStr, 64); err == nil {
					bal := int64(math.Round(v * 100))
					tx.Balance = &bal
				}
			}
		}

		out = append(out, tx)
	}

	return out, nil
}

// SBIPassword derives the standard SBI statement password from user name and date of birth.
// Password = first 5 characters of name (uppercase) + DOB in DDMMYYYY format.
func SBIPassword(name string, dob time.Time) string {
	prefix := strings.ToUpper(name)
	if len([]rune(prefix)) > 5 {
		prefix = string([]rune(prefix)[:5])
	}
	return prefix + dob.Format("02012006")
}
