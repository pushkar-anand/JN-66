package parser_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pushkaranand/finagent/internal/importer/parser"
)

func TestICICIV1_CanParse(t *testing.T) {
	p := &parser.ICICIV1{}

	assert.True(t, p.CanParse([]string{"s no.", "value date", "transaction date", "cheque number", "transaction remarks", "withdrawal amount(inr)", "deposit amount(inr)", "balance(inr)"}))
	assert.False(t, p.CanParse([]string{"tran date", "chqno", "particulars", "dr", "cr", "bal"}))
	assert.False(t, p.CanParse([]string{"date", "payment type", "transaction description", "amount"}))
}

func TestICICIV1_Parse_ErrorOnReader(t *testing.T) {
	// Parse via io.Reader is unsupported — should return error.
	p := &parser.ICICIV1{}
	_, err := p.Parse(strings.NewReader(""))
	assert.Error(t, err)
}

func TestICICIV1_Bank(t *testing.T) {
	p := &parser.ICICIV1{}
	assert.Equal(t, "icici", p.Bank())
	assert.Equal(t, "v1", p.FormatVersion())
}
