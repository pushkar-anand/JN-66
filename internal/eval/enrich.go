package eval

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pushkaranand/finagent/internal/importer"
	"github.com/pushkaranand/finagent/internal/importer/parser"
	sqlcgen "github.com/pushkaranand/finagent/internal/sqlc"
)

// EnrichEvalCase defines a single enrichment classification eval.
// All Desc, amounts, dates, and names must be anonymized fictional values.
type EnrichEvalCase struct {
	Name             string
	Desc             string // raw transaction description
	Direction        string // "debit" | "credit"
	AmountPaise      int64
	Date             time.Time
	WantCategory     string
	AllowSubcategory bool // also accept any WantCategory.* slug
}

// EnrichEvalResult holds the outcome of a single EnrichEvalCase run.
type EnrichEvalResult struct {
	Case          *EnrichEvalCase
	Passed        bool
	GotCategory   string
	GotNormalized string
	Failures      []string
	Duration      time.Duration
}

// RunEnrichEval runs a single enrichment eval case against the given Enricher.
func RunEnrichEval(ctx context.Context, e *importer.Enricher, c EnrichEvalCase) EnrichEvalResult {
	tx := parser.RawTransaction{
		Date:        c.Date,
		Description: c.Desc,
		Amount:      c.AmountPaise,
		Direction:   sqlcgen.TxnDirectionEnum(c.Direction),
	}

	t0 := time.Now()
	result, err := e.Enrich(ctx, tx)
	dur := time.Since(t0)

	res := EnrichEvalResult{Case: &c, Duration: dur}
	if err != nil {
		res.Failures = append(res.Failures, fmt.Sprintf("enrich error: %v", err))
		return res
	}

	res.GotCategory = result.CategorySlug
	res.GotNormalized = result.DescriptionNormalized

	exact := result.CategorySlug == c.WantCategory
	sub := c.AllowSubcategory && strings.HasPrefix(result.CategorySlug, c.WantCategory+".")
	if !exact && !sub {
		res.Failures = append(res.Failures, fmt.Sprintf("got: %s  want: %s", result.CategorySlug, c.WantCategory))
	}
	res.Passed = len(res.Failures) == 0
	return res
}

// EnrichScenarios is the full enrichment eval suite.
// All descriptions, amounts, dates, and names are anonymized.
var EnrichScenarios = []EnrichEvalCase{
	// Credit card payments — previously misclassified as shopping or transfer
	{
		Name:         "cc_payment_prefix",
		Desc:         "CreditCard Payment XX 1234 Ref#TESTREF000001",
		Direction:    "debit",
		AmountPaise:  500000,
		Date:         time.Date(2024, 6, 15, 0, 0, 0, 0, time.UTC),
		WantCategory: "credit_card_payment",
	},
	{
		Name:         "cc_cred_club",
		Desc:         "CRED.CLUB Payment",
		Direction:    "debit",
		AmountPaise:  1200000,
		Date:         time.Date(2024, 6, 20, 0, 0, 0, 0, time.UTC),
		WantCategory: "credit_card_payment",
	},
	{
		Name:         "cc_billdesk",
		Desc:         "American Express Credit Card Billdesk",
		Direction:    "debit",
		AmountPaise:  850000,
		Date:         time.Date(2024, 7, 5, 0, 0, 0, 0, time.UTC),
		WantCategory: "credit_card_payment",
	},
	// Bank charges — previously misclassified as tax_payment or transfer
	{
		Name:         "bank_sms_charges",
		Desc:         "SMSChgsJan24-Mar24+GST",
		Direction:    "debit",
		AmountPaise:  15000,
		Date:         time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
		WantCategory: "bank_charges",
	},
	{
		Name:         "bank_mab_penalty",
		Desc:         "MAB Charges Apr24",
		Direction:    "debit",
		AmountPaise:  50000,
		Date:         time.Date(2024, 5, 3, 0, 0, 0, 0, time.UTC),
		WantCategory: "bank_charges",
	},
	{
		Name:         "bank_debit_card_fee",
		Desc:         "DCARDFEE2615DEC23-NOV24+GST",
		Direction:    "debit",
		AmountPaise:  80000,
		Date:         time.Date(2024, 1, 15, 0, 0, 0, 0, time.UTC),
		WantCategory: "bank_charges",
	},
	{
		Name:         "bank_card_gst",
		Desc:         "Dr Card Charges GST ISSUE 0000XXX",
		Direction:    "debit",
		AmountPaise:  30000,
		Date:         time.Date(2024, 3, 10, 0, 0, 0, 0, time.UTC),
		WantCategory: "bank_charges",
	},
	{
		Name:         "bank_maintenance",
		Desc:         "Account Maintenance Fee",
		Direction:    "debit",
		AmountPaise:  25000,
		Date:         time.Date(2024, 2, 28, 0, 0, 0, 0, time.UTC),
		WantCategory: "bank_charges",
	},
	// Investment — SIP via ACH/NACH, previously misclassified as transfer
	{
		Name:         "sip_iccl",
		Desc:         "ACH Debit - Indian Clearing Corp",
		Direction:    "debit",
		AmountPaise:  500000,
		Date:         time.Date(2024, 1, 10, 0, 0, 0, 0, time.UTC),
		WantCategory: "investment.sip",
	},
	// Food — bakery, previously misclassified as transfer
	{
		Name:             "food_bakery",
		Desc:             "ABC BAKERY",
		Direction:        "debit",
		AmountPaise:      20000,
		Date:             time.Date(2024, 3, 20, 0, 0, 0, 0, time.UTC),
		WantCategory:     "food_drinks",
		AllowSubcategory: true,
	},
	// Househelp — cook payment by name, previously misclassified as transfer
	{
		Name:             "househelp_cook",
		Desc:             "ALICE SHARMA COOK",
		Direction:        "debit",
		AmountPaise:      300000,
		Date:             time.Date(2024, 3, 31, 0, 0, 0, 0, time.UTC),
		WantCategory:     "househelp",
		AllowSubcategory: true,
	},
	// Income categories
	{
		Name:         "salary",
		Desc:         "SALARY CREDIT XYZ TECHNOLOGIES PVT LTD",
		Direction:    "credit",
		AmountPaise:  5000000,
		Date:         time.Date(2024, 4, 30, 0, 0, 0, 0, time.UTC),
		WantCategory: "salary",
	},
	{
		Name:         "interest_fd",
		Desc:         "Account Int.Pd:01-01-2024 to 31-03-2024",
		Direction:    "credit",
		AmountPaise:  125000,
		Date:         time.Date(2024, 4, 1, 0, 0, 0, 0, time.UTC),
		WantCategory: "interest",
	},
	{
		Name:         "refund",
		Desc:         "REFUND ONLINE STORE ORD#TESTORDER001",
		Direction:    "credit",
		AmountPaise:  199900,
		Date:         time.Date(2024, 5, 12, 0, 0, 0, 0, time.UTC),
		WantCategory: "refund",
	},
	{
		Name:         "tax_refund",
		Desc:         "IT REFUND AY 2024-25",
		Direction:    "credit",
		AmountPaise:  1500000,
		Date:         time.Date(2024, 8, 10, 0, 0, 0, 0, time.UTC),
		WantCategory: "tax_refund",
	},
	// Transport
	{
		Name:         "cab",
		Desc:         "Uber India",
		Direction:    "debit",
		AmountPaise:  35000,
		Date:         time.Date(2024, 4, 5, 0, 0, 0, 0, time.UTC),
		WantCategory: "transport.cab",
	},
	// Utilities
	{
		Name:         "electricity",
		Desc:         "BESCOM Bill Payment",
		Direction:    "debit",
		AmountPaise:  250000,
		Date:         time.Date(2024, 4, 15, 0, 0, 0, 0, time.UTC),
		WantCategory: "utilities.electricity",
	},
	// Retail / entertainment / cash
	{
		Name:         "shopping",
		Desc:         "AMAZON SELLER SERVICES",
		Direction:    "debit",
		AmountPaise:  89900,
		Date:         time.Date(2024, 4, 8, 0, 0, 0, 0, time.UTC),
		WantCategory: "shopping",
	},
	{
		Name:         "streaming",
		Desc:         "NETFLIX.COM",
		Direction:    "debit",
		AmountPaise:  64900,
		Date:         time.Date(2024, 4, 3, 0, 0, 0, 0, time.UTC),
		WantCategory: "entertainment",
	},
	{
		Name:         "atm",
		Desc:         "ATM CASH WITHDRAWAL",
		Direction:    "debit",
		AmountPaise:  500000,
		Date:         time.Date(2024, 4, 20, 0, 0, 0, 0, time.UTC),
		WantCategory: "atm_cash",
	},
	// Investment — direct equity
	{
		Name:             "investment_equity",
		Desc:             "ZERODHA BROKING LTD",
		Direction:        "debit",
		AmountPaise:      1000000,
		Date:             time.Date(2024, 4, 25, 0, 0, 0, 0, time.UTC),
		WantCategory:     "investment",
		AllowSubcategory: true,
	},
	// Transfers / insurance
	{
		Name:         "self_transfer",
		Desc:         "NEFT Transfer Own Account Savings to FD",
		Direction:    "debit",
		AmountPaise:  2000000,
		Date:         time.Date(2024, 5, 1, 0, 0, 0, 0, time.UTC),
		WantCategory: "self_transfer",
	},
	{
		Name:         "insurance",
		Desc:         "LIC PREMIUM PAYMENT",
		Direction:    "debit",
		AmountPaise:  250000,
		Date:         time.Date(2024, 4, 10, 0, 0, 0, 0, time.UTC),
		WantCategory: "insurance",
	},
}
