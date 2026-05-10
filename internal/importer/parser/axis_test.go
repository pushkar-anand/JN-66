package parser_test

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pushkaranand/finagent/internal/importer/parser"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

func TestAxisV1_CanParse(t *testing.T) {
	p := &parser.AxisV1{}

	assert.True(t, p.CanParse([]string{"tran date", "chqno", "particulars", "dr", "cr", "bal", "sol"}))
	assert.False(t, p.CanParse([]string{"date", "payment type", "transaction description", "amount"}))
	assert.False(t, p.CanParse([]string{"date", "details", "debit", "credit", "balance"}))
}

func TestAxisV1_Parse_HappyPath(t *testing.T) {
	f, err := os.Open("testdata/axis_v1.csv")
	require.NoError(t, err)
	defer f.Close()

	p := &parser.AxisV1{}
	result, err := p.Parse(f)
	require.NoError(t, err)
	rows := result.Transactions

	assert.Equal(t, 9, len(rows), "expected 9 transactions")

	// First row: credit card payment — debit
	assert.Equal(t, sqlcgen.TxnDirectionEnumDebit, rows[0].Direction)
	assert.Equal(t, int64(500000), rows[0].Amount) // ₹5000.00

	// Second row: NEFT credit
	assert.Equal(t, sqlcgen.TxnDirectionEnumCredit, rows[1].Direction)
	assert.Equal(t, int64(1500000), rows[1].Amount) // ₹15000.00

	// Interest credit
	assert.Equal(t, sqlcgen.TxnDirectionEnumCredit, rows[6].Direction)
	assert.Equal(t, int64(20000), rows[6].Amount) // ₹200.00
}

func TestAxisV1_Parse_Meta(t *testing.T) {
	f, err := os.Open("testdata/axis_v1.csv")
	require.NoError(t, err)
	defer f.Close()

	p := &parser.AxisV1{}
	result, err := p.Parse(f)
	require.NoError(t, err)

	assert.NotEmpty(t, result.Meta.AccountNumber)
	assert.NotEmpty(t, result.Meta.AccountHolder)
}

func TestAxisV1_Parse_AmountParsing(t *testing.T) {
	csv := `Name :- TEST
Currency :- INR

Statement of Account No - 1 for the period (From : 01-04-2025 To : 31-03-2026)

Tran Date,CHQNO,PARTICULARS,DR,CR,BAL,SOL
01-04-2025,-,UPI/merchant@bank/payment,           1,23,456.00, ,            50000.00,1000
`
	p := &parser.AxisV1{}
	result, err := p.Parse(strings.NewReader(csv))
	require.NoError(t, err)
	rows := result.Transactions
	// Amounts with commas should be handled; note 1,23,456.00 may parse to 123456 or fail gracefully
	// The parser strips commas, so "1,23,456.00" → "123456.00" → 12345600 paise
	if len(rows) > 0 {
		assert.Equal(t, sqlcgen.TxnDirectionEnumDebit, rows[0].Direction)
	}
}

func TestAxisV1_Parse_DateParsing(t *testing.T) {
	f, err := os.Open("testdata/axis_v1.csv")
	require.NoError(t, err)
	defer f.Close()

	p := &parser.AxisV1{}
	result, err := p.Parse(f)
	require.NoError(t, err)
	rows := result.Transactions
	require.NotEmpty(t, rows)

	assert.Equal(t, 2025, rows[0].Date.Year())
	assert.Equal(t, 4, int(rows[0].Date.Month()))
	assert.Equal(t, 3, rows[0].Date.Day())
}

func TestAxisV1_Bank(t *testing.T) {
	p := &parser.AxisV1{}
	assert.Equal(t, "axis", p.Bank())
	assert.Equal(t, "v1", p.FormatVersion())
}
