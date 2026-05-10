package parser_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pushkaranand/finagent/internal/importer/parser"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

func TestSBIV1_CanParse(t *testing.T) {
	p := &parser.SBIV1{}

	assert.True(t, p.CanParse([]string{"date", "details", "ref no/cheque no", "debit", "credit", "balance"}))
	assert.False(t, p.CanParse([]string{"tran date", "chqno", "particulars", "dr", "cr", "bal"}))
	assert.False(t, p.CanParse([]string{"date", "payment type", "transaction description", "amount"}))
}

func TestSBIV1_ParseXLSX_HappyPath(t *testing.T) {
	// Uses an unencrypted XLSX fixture (empty password) to test the real XLSX path.
	p := &parser.SBIV1{}
	rows, err := p.ParseXLSX("testdata/sbi_v1.xlsx", "")
	require.NoError(t, err)

	assert.Equal(t, 6, len(rows))

	// First row: insurance premium — credit
	assert.Equal(t, sqlcgen.TxnDirectionEnumCredit, rows[0].Direction)
	assert.Equal(t, int64(19999), rows[0].Amount) // ₹199.99

	// Second row: PMSBY debit
	assert.Equal(t, sqlcgen.TxnDirectionEnumDebit, rows[1].Direction)
	assert.Equal(t, int64(2000), rows[1].Amount) // ₹20.00
}

func TestSBIV1_ParseXLSX_DateParsing(t *testing.T) {
	p := &parser.SBIV1{}
	rows, err := p.ParseXLSX("testdata/sbi_v1.xlsx", "")
	require.NoError(t, err)
	require.NotEmpty(t, rows)

	assert.Equal(t, 2025, rows[0].Date.Year())
	assert.Equal(t, 4, int(rows[0].Date.Month()))
	assert.Equal(t, 8, rows[0].Date.Day())
}

func TestSBIV1_Parse_CSV_FallbackForLogicTest(t *testing.T) {
	// Tests row-parsing logic via plain CSV (no XLSX encryption layer).
	f, err := os.Open("testdata/sbi_v1.csv")
	require.NoError(t, err)
	defer f.Close()

	p := &parser.SBIV1{}
	rows, err := p.Parse(f)
	require.NoError(t, err)
	assert.Equal(t, 6, len(rows))
}

func TestSBIPassword(t *testing.T) {
	dob := time.Date(1999, 3, 3, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, "PUSHK03031999", parser.SBIPassword("Pushkar Anand", dob))
	assert.Equal(t, "RAHUL01011990", parser.SBIPassword("Rahul Sharma", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)))
	// Name shorter than 5 chars
	assert.Equal(t, "ANU01011990", parser.SBIPassword("Anu", time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)))
}

func TestSBIV1_Bank(t *testing.T) {
	p := &parser.SBIV1{}
	assert.Equal(t, "sbi", p.Bank())
	assert.Equal(t, "v1", p.FormatVersion())
}
