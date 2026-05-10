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
)

// IDFCV1 parses IDFC First Bank CSV statements (format v1).
// Header at row 1: Date, Payment type, Transaction description, Category, Sub-category, Amount, Currency
// Amount is a signed float: negative = debit, positive = credit.
type IDFCV1 struct{}

func (p *IDFCV1) Bank() string          { return "idfc" }
func (p *IDFCV1) FormatVersion() string { return "v1" }

func (p *IDFCV1) CanParse(header []string) bool {
	has := func(s string) bool {
		for _, h := range header {
			if strings.Contains(h, s) {
				return true
			}
		}
		return false
	}
	return has("payment type") && has("transaction description") && has("amount")
}

// Parse implements Parser. IDFC CSV has no account metadata in the file.
func (p *IDFCV1) Parse(r io.Reader) (ParseResult, error) {
	rdr := csv.NewReader(r)
	rdr.LazyQuotes = true
	rdr.FieldsPerRecord = -1

	var (
		headerFound bool
		colDate     int
		colDesc     int
		colAmount   int
	)

	var rows []RawTransaction
	lineNum := 0

	for {
		record, err := rdr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return ParseResult{}, fmt.Errorf("idfc csv read line %d: %w", lineNum, err)
		}
		lineNum++

		if !headerFound {
			norm := normalise(record)
			if findCol(norm, "payment type") >= 0 {
				headerFound = true
				colDate = findCol(norm, "date")
				colDesc = findCol(norm, "transaction description")
				colAmount = findCol(norm, "amount")
			}
			continue
		}

		if len(record) == 0 {
			continue
		}

		rawDate := strings.Trim(strings.TrimSpace(safeCol(record, colDate)), `"`)
		date, err := parseIDFCDate(rawDate)
		if err != nil {
			continue
		}

		desc := strings.TrimSpace(safeCol(record, colDesc))
		amtStr := cleanAmount(safeCol(record, colAmount))
		if amtStr == "" {
			continue
		}

		v, err := strconv.ParseFloat(amtStr, 64)
		if err != nil {
			continue
		}

		// IDFC uses signed amounts: negative = money out (debit), positive = money in (credit).
		var dir sqlcgen.TxnDirectionEnum
		if v < 0 {
			dir = sqlcgen.TxnDirectionEnumDebit
			v = -v
		} else {
			dir = sqlcgen.TxnDirectionEnumCredit
		}

		rows = append(rows, RawTransaction{
			Date:        date,
			Description: desc,
			Amount:      int64(math.Round(v * 100)),
			Direction:   dir,
		})
	}

	return ParseResult{Transactions: rows}, nil
}

// parseIDFCDate parses IDFC date strings like "6 May, 2026" or "30 Apr, 2025".
func parseIDFCDate(s string) (time.Time, error) {
	// Remove comma after day to normalise "6 May, 2026" → "6 May 2026"
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSpace(s)
	t, err := time.Parse("2 Jan 2006", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("idfc date %q: %w", s, err)
	}
	return t, nil
}
