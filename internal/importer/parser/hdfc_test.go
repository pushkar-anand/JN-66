package parser_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/pushkaranand/finagent/internal/importer/parser"
)

func TestHDFCV1_CanParse(t *testing.T) {
	p := &parser.HDFCV1{}

	assert.True(t, p.CanParse([]string{"date", "narration", "chq./ref.no.", "value dt", "withdrawal amt.", "deposit amt.", "closing balance"}))
	assert.False(t, p.CanParse([]string{"date", "payment type", "transaction description", "amount"}))
	assert.False(t, p.CanParse([]string{"s no.", "value date", "transaction date", "cheque number", "transaction remarks", "withdrawal amount(inr)", "deposit amount(inr)", "balance(inr)"}))
	assert.False(t, p.CanParse([]string{"tran date", "chqno", "particulars", "dr", "cr", "bal"}))
}

func TestHDFCV1_Parse_ErrorOnReader(t *testing.T) {
	p := &parser.HDFCV1{}
	_, err := p.Parse(strings.NewReader(""))
	assert.Error(t, err)
}

func TestHDFCV1_Bank(t *testing.T) {
	p := &parser.HDFCV1{}
	assert.Equal(t, "hdfc", p.Bank())
	assert.Equal(t, "v1", p.FormatVersion())
}
