// Package parser provides bank statement parsers that produce a common RawTransaction type.
package parser

import (
	"io"
	"strings"
	"time"

	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// RawTransaction is the bank-agnostic output of all parsers.
type RawTransaction struct {
	Date        time.Time
	Description string
	Amount      int64 // paise (INR × 100), always positive
	Direction   sqlcgen.TxnDirectionEnum
	Reference   string // UTR / cheque number, may be empty
	Balance     *int64 // closing balance in paise, optional
}

// StatementMeta holds account-level metadata extracted from the statement file.
// Fields are best-effort; parsers leave fields empty when not present in the file.
type StatementMeta struct {
	AccountNumber string // full account number as printed on statement
	AccountHolder string // name of the account holder
	IFSC          string // branch IFSC code
	Currency      string // ISO 4217, e.g. "INR"
}

// ParseResult is the full output of a parser: account metadata and transactions.
type ParseResult struct {
	Meta         StatementMeta
	Transactions []RawTransaction
}

// Parser reads a bank statement and returns a ParseResult.
// Implementations must be registered in the Registry so auto-detection works.
type Parser interface {
	// CanParse reports whether this parser handles the given header row.
	// header is a slice of lowercase, trimmed column names.
	CanParse(header []string) bool
	// Parse reads all transactions and any account metadata from r.
	Parse(r io.Reader) (ParseResult, error)
	// Bank returns the bank identifier (e.g. "axis", "idfc").
	Bank() string
	// FormatVersion returns the format version string (e.g. "v1") for diagnostics.
	FormatVersion() string
}

// detectPaymentMode guesses the payment mode from the raw transaction description.
func DetectPaymentMode(desc string) *sqlcgen.PaymentModeEnum {
	u := strings.ToUpper(desc)
	var m sqlcgen.PaymentModeEnum
	switch {
	case strings.HasPrefix(u, "UPI/AUTOPAY") || strings.HasPrefix(u, "UPI-AUTOPAY") ||
		strings.Contains(u, "AUTOPAY") && strings.Contains(u, "UPI"):
		m = sqlcgen.PaymentModeEnumUpiAutopay
	case strings.HasPrefix(u, "UPI/") || strings.HasPrefix(u, "UPI-") ||
		strings.Contains(u, "@") && !strings.Contains(u, "NEFT") && !strings.Contains(u, "IMPS"):
		m = sqlcgen.PaymentModeEnumUpi
	case strings.HasPrefix(u, "NEFT/") || strings.HasPrefix(u, "NEFT-") || strings.HasPrefix(u, "NEFT "):
		m = sqlcgen.PaymentModeEnumNeft
	case strings.HasPrefix(u, "RTGS/") || strings.HasPrefix(u, "RTGS-"):
		m = sqlcgen.PaymentModeEnumRtgs
	case strings.HasPrefix(u, "IMPS/") || strings.HasPrefix(u, "IMPS-") ||
		strings.HasPrefix(u, "MMT/IMPS"):
		m = sqlcgen.PaymentModeEnumImps
	case strings.HasPrefix(u, "NACH/") || strings.HasPrefix(u, "NACH-") ||
		strings.Contains(u, "NACH"):
		m = sqlcgen.PaymentModeEnumNach
	case strings.HasPrefix(u, "ATM") || strings.Contains(u, "CWDR") || strings.Contains(u, "ATM CASH"):
		m = sqlcgen.PaymentModeEnumAtm
	case strings.HasPrefix(u, "POS/") || strings.HasPrefix(u, "PUR ") || strings.Contains(u, "POS "):
		m = sqlcgen.PaymentModeEnumPos
	case strings.Contains(u, "EMI"):
		m = sqlcgen.PaymentModeEnumEmi
	default:
		return nil
	}
	return &m
}
