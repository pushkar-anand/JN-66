package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMoney_Rupees(t *testing.T) {
	assert.Equal(t, 100.0, Money(10000).Rupees())
	assert.Equal(t, 0.5, Money(50).Rupees())
	assert.Equal(t, 0.0, Money(0).Rupees())
}

func TestMoney_String(t *testing.T) {
	cases := []struct {
		m    Money
		want string
	}{
		{10000, "₹100.00"},
		{50, "₹0.50"},
		{0, "₹0.00"},
		{64900, "₹649.00"},
		{123456, "₹1234.56"},
		{-10000, "-₹100.00"},
	}
	for _, c := range cases {
		assert.Equal(t, c.want, c.m.String(), "Money(%d)", int64(c.m))
	}
}

func TestFromRupees(t *testing.T) {
	assert.Equal(t, Money(10000), FromRupees(100))
	assert.Equal(t, Money(64900), FromRupees(649))
	assert.Equal(t, Money(0), FromRupees(0))
}
