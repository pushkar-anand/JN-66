// Package model contains shared domain types.
package model

import (
	"fmt"

	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// Money represents an INR amount stored as paise (INR × 100).
// Always positive; direction (debit/credit) is stored separately.
type Money int64

// inrPrinter formats integers with en-IN locale grouping (Indian comma style).
var inrPrinter = message.NewPrinter(language.Make("en-IN"))

// Rupees returns the amount in rupees as a float64.
func (m Money) Rupees() float64 { return float64(m) / 100 }

// String formats as "₹1,00,000.00" using Indian comma grouping.
func (m Money) String() string {
	r := int64(m)
	sign := ""
	if r < 0 {
		sign = "-"
		r = -r
	}
	rupees := r / 100
	paise := r % 100
	return fmt.Sprintf("%s₹%s.%02d", sign, inrPrinter.Sprintf("%d", rupees), paise)
}

// FromRupees converts a rupee float to Money (paise).
func FromRupees(r float64) Money { return Money(r * 100) }
