package ledger

import (
	"errors"
	"strconv"
	"strings"
)

// Money is an amount in a currency's minor units (e.g. cents when decimals=2).
// It is an integer to avoid floating-point rounding on monetary values.
type Money int64

var errBadMoney = errors.New("invalid money value")

// ParseMoney parses a decimal string like "12.34" or "-5" into Money using
// `decimals` fractional digits (2 for USD/EUR, 0 for JPY/IDR). It rejects more
// fractional digits than `decimals` rather than silently rounding money away.
func ParseMoney(s string, decimals int) (Money, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, errBadMoney
	}
	neg := false
	switch s[0] {
	case '-':
		neg, s = true, s[1:]
	case '+':
		s = s[1:]
	}
	intPart, fracPart := s, ""
	if i := strings.IndexByte(s, '.'); i >= 0 {
		intPart, fracPart = s[:i], s[i+1:]
	}
	if len(fracPart) > decimals {
		return 0, errBadMoney
	}
	// Reject anything that isn't purely digits in the integer and fraction parts:
	// this catches "-", "+", ".", and embedded signs like "+-5" that strconv would
	// otherwise silently accept as 0 or a sign-flipped value.
	if !allDigits(intPart) || !allDigits(fracPart) || intPart+fracPart == "" {
		return 0, errBadMoney
	}
	fracPart += strings.Repeat("0", decimals-len(fracPart))
	n, err := strconv.ParseInt(intPart+fracPart, 10, 64)
	if err != nil {
		return 0, errBadMoney
	}
	if neg {
		n = -n
	}
	return Money(n), nil
}

// Format renders Money with `decimals` fractional digits, e.g. 1234 -> "12.34".
// It formats the magnitude as a digit string (never negating the int64) so it is
// correct even at math.MinInt64.
func (m Money) Format(decimals int) string {
	s := strconv.FormatInt(int64(m), 10)
	if decimals <= 0 {
		return s
	}
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	for len(s) <= decimals { // ensure at least one leading digit before the point
		s = "0" + s
	}
	point := len(s) - decimals
	out := s[:point] + "." + s[point:]
	if neg {
		out = "-" + out
	}
	return out
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
