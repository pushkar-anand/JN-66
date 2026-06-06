// Package model contains shared domain types.
package model

import (
	"fmt"
	"strconv"
)

// Money represents an INR amount stored as paise (INR × 100).
// Always positive; direction (debit/credit) is stored separately.
type Money int64

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
	return fmt.Sprintf("%s₹%s.%02d", sign, indianComma(rupees), paise)
}

// indianComma formats n with Indian grouping: last group of 3, then groups of 2.
// E.g. 100000 → "1,00,000";  50000 → "50,000".
func indianComma(n int64) string {
	s := strconv.FormatInt(n, 10)
	if len(s) <= 3 {
		return s
	}
	result := s[len(s)-3:]
	s = s[:len(s)-3]
	for len(s) > 2 {
		result = s[len(s)-2:] + "," + result
		s = s[:len(s)-2]
	}
	return s + "," + result
}

// FromRupees converts a rupee float to Money (paise).
func FromRupees(r float64) Money { return Money(r * 100) }
