package ledger

import "testing"

func TestParseFormatRoundTrip(t *testing.T) {
	cases := []struct {
		in       string
		decimals int
		want     Money
		str      string
	}{
		{"12.34", 2, 1234, "12.34"},
		{"-5", 2, -500, "-5.00"},
		{"0", 2, 0, "0.00"},
		{"1000", 0, 1000, "1000"},
		{".5", 2, 50, "0.50"},
		{"12.", 2, 1200, "12.00"},
		{"+7.08", 2, 708, "7.08"},
	}
	for _, c := range cases {
		got, err := ParseMoney(c.in, c.decimals)
		if err != nil {
			t.Fatalf("ParseMoney(%q,%d): %v", c.in, c.decimals, err)
		}
		if got != c.want {
			t.Errorf("ParseMoney(%q,%d)=%d want %d", c.in, c.decimals, got, c.want)
		}
		if s := got.Format(c.decimals); s != c.str {
			t.Errorf("Format(%d,%d)=%q want %q", got, c.decimals, s, c.str)
		}
	}
}

func TestParseMoneyErrors(t *testing.T) {
	bad := []struct {
		in       string
		decimals int
	}{
		{"12.345", 2}, // too many fractional digits: no silent rounding
		{"12.5", 0},
		{"abc", 2},
		{"", 2},
		{"1.2.3", 2},
	}
	for _, c := range bad {
		if _, err := ParseMoney(c.in, c.decimals); err == nil {
			t.Errorf("ParseMoney(%q,%d) expected error", c.in, c.decimals)
		}
	}
}
