package parser_test

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/pushkaranand/finagent/internal/importer/parser"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

func TestIDFCV1_CanParse(t *testing.T) {
	p := &parser.IDFCV1{}

	assert.True(t, p.CanParse([]string{"date", "payment type", "transaction description", "category", "sub-category", "amount", "currency"}))
	assert.False(t, p.CanParse([]string{"tran date", "chqno", "particulars", "dr", "cr", "bal"}))
	assert.False(t, p.CanParse([]string{"date", "details", "debit", "credit", "balance"}))
}

func TestIDFCV1_Parse_HappyPath(t *testing.T) {
	f, err := os.Open("testdata/idfc_v1.csv")
	require.NoError(t, err)
	defer f.Close()

	p := &parser.IDFCV1{}
	result, err := p.Parse(f)
	require.NoError(t, err)
	rows := result.Transactions

	assert.Equal(t, 8, len(rows))

	// First row: outgoing transfer — debit
	assert.Equal(t, sqlcgen.TxnDirectionEnumDebit, rows[0].Direction)
	assert.Equal(t, int64(8000000), rows[0].Amount) // ₹80,000.00

	// Salary credit
	assert.Equal(t, sqlcgen.TxnDirectionEnumCredit, rows[2].Direction)
	assert.Equal(t, int64(23645100), rows[2].Amount) // ₹2,36,451.00

	// Interest credit
	assert.Equal(t, sqlcgen.TxnDirectionEnumCredit, rows[3].Direction)
	assert.Equal(t, int64(4200), rows[3].Amount) // ₹42.00
}

func TestIDFCV1_Parse_DateParsing(t *testing.T) {
	f, err := os.Open("testdata/idfc_v1.csv")
	require.NoError(t, err)
	defer f.Close()

	p := &parser.IDFCV1{}
	result, err := p.Parse(f)
	require.NoError(t, err)
	rows := result.Transactions
	require.NotEmpty(t, rows)

	assert.Equal(t, 2026, rows[0].Date.Year())
	assert.Equal(t, 5, int(rows[0].Date.Month()))
	assert.Equal(t, 6, rows[0].Date.Day())
}

func TestIDFCV1_Bank(t *testing.T) {
	p := &parser.IDFCV1{}
	assert.Equal(t, "idfc", p.Bank())
	assert.Equal(t, "v1", p.FormatVersion())
}
