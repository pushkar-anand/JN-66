// Package model contains shared domain types.
package model

import "fmt"

// Money represents an INR amount stored as paise (INR × 100).
// Always positive; direction (debit/credit) is stored separately.
type Money int64

// Rupees returns the amount in rupees as a float64.
func (m Money) Rupees() float64 { return float64(m) / 100 }

// String formats as "₹1,234.56".
func (m Money) String() string {
	r := int64(m)
	sign := ""
	if r < 0 {
		sign = "-"
		r = -r
	}
	rupees := r / 100
	paise := r % 100
	return fmt.Sprintf("%s₹%d.%02d", sign, rupees, paise)
}

// FromRupees converts a rupee float to Money (paise).
func FromRupees(r float64) Money { return Money(r * 100) }
