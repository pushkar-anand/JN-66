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

// ICICIV1 parses ICICI Bank binary XLS statements (format v1).
// Header at row 13 (0-indexed: 12): S No., Value Date, Transaction Date, Cheque Number,
// Transaction Remarks, Withdrawal Amount(INR), Deposit Amount(INR), Balance(INR)
// Data columns start at index 1 (column B). Footer begins with "Legends Used".
type ICICIV1 struct{}

func (p *ICICIV1) Bank() string          { return "icici" }
func (p *ICICIV1) FormatVersion() string { return "v1" }

func (p *ICICIV1) CanParse(header []string) bool {
	has := func(s string) bool {
		for _, h := range header {
			if strings.Contains(h, s) {
				return true
			}
		}
		return false
	}
	return has("transaction remarks") && has("withdrawal amount") && has("deposit amount")
}

// Parse implements Parser. ICICI XLS requires a file path; this always returns an error.
// Use ParsePath when working with real files.
func (p *ICICIV1) Parse(r io.Reader) (ParseResult, error) {
	return ParseResult{}, fmt.Errorf("icici: use ParsePath to read XLS files directly")
}

// ParsePath reads an ICICI XLS statement from a file path.
func (p *ICICIV1) ParsePath(path string) (ParseResult, error) {
	slog.Debug("parser start", slog.String("bank", p.Bank()), slog.String("format", p.FormatVersion()))
	wb, err := xlslib.Open(path, "utf-8")
	if err != nil {
		return ParseResult{}, fmt.Errorf("icici open xls: %w", err)
	}
	sheet := wb.GetSheet(0)
	if sheet == nil {
		return ParseResult{}, fmt.Errorf("icici: no sheets found")
	}

	var (
		headerFound bool
		colDate     int // Transaction Date
		colDesc     int // Transaction Remarks
		colWithdraw int // Withdrawal Amount
		colDeposit  int // Deposit Amount
		colBal      int // Balance
		colRef      int // Cheque Number
		meta        StatementMeta
	)

	var out []RawTransaction

	for rowIdx := 0; rowIdx <= int(sheet.MaxRow); rowIdx++ {
		row := sheet.Row(rowIdx)
		if row == nil {
			continue
		}

		if !headerFound {
			// Extract account metadata from pre-header rows.
			// Row 3 col 3: "046301004351 ( INR )  - PUSHKAR ANAND"
			if rowIdx == 3 {
				cell := strings.TrimSpace(row.Col(3))
				if cell != "" {
					extractICICIMeta(cell, &meta)
				}
			}

			// Collect all cell values from this row.
			var cells []string
			for colIdx := row.FirstCol(); colIdx <= row.LastCol(); colIdx++ {
				cells = append(cells, strings.ToLower(strings.TrimSpace(row.Col(colIdx))))
			}
			if findCol(cells, "transaction remarks") >= 0 &&
				findCol(cells, "withdrawal amount") >= 0 &&
				findCol(cells, "deposit amount") >= 0 {
				headerFound = true
				// Columns start at row.FirstCol() offset.
				offset := row.FirstCol()
				norm := cells
				colDate = findCol(norm, "transaction date") + offset
				colRef = findCol(norm, "cheque number") + offset
				colDesc = findCol(norm, "transaction remarks") + offset
				colWithdraw = findCol(norm, "withdrawal amount") + offset
				colDeposit = findCol(norm, "deposit amount") + offset
				colBal = findCol(norm, "balance") + offset
			}
			continue
		}

		// Footer starts when col 1 contains "Legends" (col 0 is always empty in this format).
		if strings.Contains(strings.ToLower(strings.TrimSpace(row.Col(1))), "legend") {
			break
		}

		rawDate := strings.TrimSpace(row.Col(colDate))
		if rawDate == "" {
			continue
		}
		date, err := time.Parse("02/01/2006", rawDate)
		if err != nil {
			continue
		}

		desc := strings.TrimSpace(row.Col(colDesc))
		ref := strings.TrimSpace(row.Col(colRef))
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

// extractICICIMeta parses the ICICI account number and holder from the combined cell string.
// Format: "046301004351 ( INR )  - PUSHKAR ANAND"
func extractICICIMeta(cell string, meta *StatementMeta) {
	// Split on " - " to get account part and name.
	parts := strings.SplitN(cell, " - ", 2)
	if len(parts) == 2 {
		// Account part may be "046301004351 ( INR )" — take the first token.
		meta.AccountNumber = strings.Fields(parts[0])[0]
		meta.AccountHolder = strings.TrimSpace(parts[1])
		// Extract currency if present.
		if strings.Contains(parts[0], "INR") {
			meta.Currency = "INR"
		}
	}
}
