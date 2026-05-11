package parser

import (
	"fmt"
	"io"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	xlslib "github.com/extrame/xls"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// HDFCV1 parses HDFC Bank binary XLS statements (format v1).
// Header at row 20 (0-indexed): Date, Narration, Chq./Ref.No., Value Dt, Withdrawal Amt., Deposit Amt., Closing Balance
// Data starts at row 22. Dates are DD/MM/YY (two-digit year).
type HDFCV1 struct{}

func (p *HDFCV1) Bank() string          { return "hdfc" }
func (p *HDFCV1) FormatVersion() string { return "v1" }

func (p *HDFCV1) CanParse(header []string) bool {
	has := func(s string) bool {
		for _, h := range header {
			if strings.Contains(h, s) {
				return true
			}
		}
		return false
	}
	return has("narration") && has("withdrawal amt") && has("deposit amt")
}

// Parse implements Parser. HDFC XLS requires a file path; this always returns an error.
func (p *HDFCV1) Parse(_ io.Reader) (ParseResult, error) {
	return ParseResult{}, fmt.Errorf("hdfc: use ParsePath to read XLS files directly")
}

// ParsePath reads an HDFC XLS statement from a file path.
func (p *HDFCV1) ParsePath(path string) (ParseResult, error) {
	slog.Debug("parser start", slog.String("bank", p.Bank()), slog.String("format", p.FormatVersion()))
	wb, err := xlslib.Open(path, "utf-8")
	if err != nil {
		return ParseResult{}, fmt.Errorf("hdfc open xls: %w", err)
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return ParseResult{}, fmt.Errorf("hdfc: no sheets found")
	}

	var (
		headerFound bool
		colDate     int
		colDesc     int
		colRef      int
		colWithdraw int
		colDeposit  int
		colBal      int
		meta        StatementMeta
	)

	var out []RawTransaction

	for rowIdx := 0; rowIdx <= int(sheet.MaxRow); rowIdx++ {
		row := safeXLSRow(sheet, rowIdx)
		if row == nil {
			continue
		}

		if !headerFound {
			// Extract metadata from pre-header rows.
			switch rowIdx {
			case 5:
				extractHDFCHolder(strings.TrimSpace(row.Col(0)), &meta)
			case 14:
				extractHDFCField(strings.TrimSpace(row.Col(4)), "Account No :", &meta.AccountNumber)
			case 17:
				extractHDFCField(strings.TrimSpace(row.Col(4)), "RTGS/NEFT IFSC :", &meta.IFSC)
				meta.Currency = "INR"
			}

			var cells []string
			for colIdx := row.FirstCol(); colIdx <= row.LastCol(); colIdx++ {
				cells = append(cells, strings.ToLower(strings.TrimSpace(row.Col(colIdx))))
			}
			if findCol(cells, "narration") >= 0 &&
				findCol(cells, "withdrawal amt") >= 0 &&
				findCol(cells, "deposit amt") >= 0 {
				headerFound = true
				offset := row.FirstCol()
				colDate = findCol(cells, "date") + offset
				colDesc = findCol(cells, "narration") + offset
				colRef = findCol(cells, "chq") + offset
				colWithdraw = findCol(cells, "withdrawal amt") + offset
				colDeposit = findCol(cells, "deposit amt") + offset
				colBal = findCol(cells, "closing balance") + offset
			}
			continue
		}

		rawDate := strings.TrimSpace(row.Col(colDate))
		if rawDate == "" {
			continue
		}
		date, err := time.Parse("02/01/06", rawDate)
		if err != nil {
			continue
		}

		desc := sanitizeXLSCell(row.Col(colDesc))
		ref := sanitizeXLSCell(row.Col(colRef))
		withdrawStr := cleanAmount(row.Col(colWithdraw))
		depositStr := cleanAmount(row.Col(colDeposit))

		var amount int64
		var dir sqlcgen.TxnDirectionEnum

		switch {
		case withdrawStr != "" && withdrawStr != "0" && withdrawStr != "0.00":
			v, err := strconv.ParseFloat(withdrawStr, 64)
			if err != nil || v == 0 {
				continue
			}
			amount = int64(math.Round(v * 100))
			dir = sqlcgen.TxnDirectionEnumDebit
		case depositStr != "" && depositStr != "0" && depositStr != "0.00":
			v, err := strconv.ParseFloat(depositStr, 64)
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
			balStr := cleanAmount(row.Col(colBal))
			if balStr != "" {
				if v, err := strconv.ParseFloat(balStr, 64); err == nil {
					bal := int64(math.Round(v * 100))
					tx.Balance = &bal
				}
			}
		}

		out = append(out, tx)
	}

	slog.Info("parser done", slog.String("bank", p.Bank()), slog.Int("parsed", len(out)))
	return ParseResult{Meta: meta, Transactions: out}, nil
}

// extractHDFCHolder parses the account holder name from the header cell.
// Format: "MR.     PUSHKAR ANAND" or "MRS.   SAMPLE NAME"
func extractHDFCHolder(cell string, meta *StatementMeta) {
	if cell == "" {
		return
	}
	for _, prefix := range []string{"MR.", "MRS.", "MS.", "DR."} {
		if strings.HasPrefix(strings.ToUpper(cell), prefix) {
			cell = strings.TrimSpace(cell[len(prefix):])
			break
		}
	}
	// Normalize multiple spaces.
	meta.AccountHolder = strings.Join(strings.Fields(cell), " ")
}

// extractHDFCField extracts a value after a known prefix from a cell.
// e.g. "Account No :50100379473504   PRIME" with prefix "Account No :" → "50100379473504"
func extractHDFCField(cell, prefix string, out *string) {
	idx := strings.Index(strings.ToUpper(cell), strings.ToUpper(prefix))
	if idx < 0 {
		return
	}
	val := strings.TrimSpace(cell[idx+len(prefix):])
	if f := strings.Fields(val); len(f) > 0 {
		*out = f[0]
	}
}
