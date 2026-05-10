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

// AxisV1 parses Axis Bank savings/salary/current CSV statements (format v1).
// Header row: Tran Date, CHQNO, PARTICULARS, DR, CR, BAL, SOL
// Up to ~18 metadata rows precede the header; footer rows follow the last data row.
type AxisV1 struct{}

func (p *AxisV1) Bank() string          { return "axis" }
func (p *AxisV1) FormatVersion() string { return "v1" }

func (p *AxisV1) CanParse(header []string) bool {
	has := func(s string) bool {
		for _, h := range header {
			if strings.Contains(h, s) {
				return true
			}
		}
		return false
	}
	return has("particulars") && has("dr") && has("cr") && has("tran date")
}

func (p *AxisV1) Parse(r io.Reader) ([]RawTransaction, error) {
	rdr := csv.NewReader(r)
	rdr.LazyQuotes = true
	rdr.FieldsPerRecord = -1

	var (
		headerIdx = -1
		colDate   int
		colDesc   int
		colDR     int
		colCR     int
		colBAL    int
		colRef    int
	)

	var rows []RawTransaction
	lineNum := 0

	for {
		record, err := rdr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("axis csv read line %d: %w", lineNum, err)
		}
		lineNum++

		if headerIdx == -1 {
			// Find header row by looking for known column names.
			norm := normalise(record)
			if idx := findCol(norm, "tran date"); idx >= 0 {
				headerIdx = lineNum
				colDate = idx
				colDesc = findCol(norm, "particulars")
				colDR = findCol(norm, "dr")
				colCR = findCol(norm, "cr")
				colBAL = findCol(norm, "bal")
				colRef = findCol(norm, "chqno")
			}
			continue
		}

		// Stop at footer (empty or disclaimer rows).
		if len(record) == 0 || strings.TrimSpace(record[0]) == "" {
			continue
		}
		// Footer signal: row doesn't look like a date.
		if !looksLikeAxisDate(strings.TrimSpace(record[colDate])) {
			break
		}

		date, err := time.Parse("02-01-2006", strings.TrimSpace(record[colDate]))
		if err != nil {
			continue
		}

		desc := strings.TrimSpace(safeCol(record, colDesc))
		drStr := cleanAmount(safeCol(record, colDR))
		crStr := cleanAmount(safeCol(record, colCR))
		ref := strings.TrimSpace(safeCol(record, colRef))
		if ref == "-" {
			ref = ""
		}

		var amount int64
		var dir sqlcgen.TxnDirectionEnum

		switch {
		case drStr != "" && drStr != "0" && drStr != "0.00":
			v, err := strconv.ParseFloat(drStr, 64)
			if err != nil {
				continue
			}
			amount = int64(math.Round(v * 100))
			dir = sqlcgen.TxnDirectionEnumDebit
		case crStr != "" && crStr != "0" && crStr != "0.00":
			v, err := strconv.ParseFloat(crStr, 64)
			if err != nil {
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

		if colBAL >= 0 {
			balStr := cleanAmount(safeCol(record, colBAL))
			if balStr != "" {
				if v, err := strconv.ParseFloat(balStr, 64); err == nil {
					bal := int64(math.Round(v * 100))
					tx.Balance = &bal
				}
			}
		}

		rows = append(rows, tx)
	}

	return rows, nil
}

func looksLikeAxisDate(s string) bool {
	// Axis dates are DD-MM-YYYY
	if len(s) != 10 {
		return false
	}
	_, err := time.Parse("02-01-2006", s)
	return err == nil
}

// normalise lowercases and trims each field.
func normalise(row []string) []string {
	out := make([]string, len(row))
	for i, v := range row {
		out[i] = strings.ToLower(strings.TrimSpace(v))
	}
	return out
}

// findCol returns the index of the first column containing substr, or -1.
func findCol(header []string, substr string) int {
	for i, h := range header {
		if strings.Contains(h, substr) {
			return i
		}
	}
	return -1
}

// safeCol returns record[i] or "" if i is out of range.
func safeCol(record []string, i int) string {
	if i < 0 || i >= len(record) {
		return ""
	}
	return record[i]
}

// cleanAmount strips commas, spaces, and CR suffixes from an amount string.
func cleanAmount(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, ",", "")
	s = strings.TrimSuffix(s, "CR")
	s = strings.TrimSuffix(s, "DR")
	return strings.TrimSpace(s)
}
