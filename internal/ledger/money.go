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
	fracPart += strings.Repeat("0", decimals-len(fracPart))
	digits := intPart + fracPart
	if digits == "" {
		return 0, errBadMoney
	}
	n, err := strconv.ParseInt(digits, 10, 64)
	if err != nil {
		return 0, errBadMoney
	}
	if neg {
		n = -n
	}
	return Money(n), nil
}

// Format renders Money with `decimals` fractional digits, e.g. 1234 -> "12.34".
func (m Money) Format(decimals int) string {
	n := int64(m)
	sign := ""
	if n < 0 {
		sign, n = "-", -n
	}
	if decimals == 0 {
		return sign + strconv.FormatInt(n, 10)
	}
	scale := int64(1)
	for i := 0; i < decimals; i++ {
		scale *= 10
	}
	frac := strconv.FormatInt(n%scale, 10)
	if len(frac) < decimals {
		frac = strings.Repeat("0", decimals-len(frac)) + frac
	}
	return sign + strconv.FormatInt(n/scale, 10) + "." + frac
}
