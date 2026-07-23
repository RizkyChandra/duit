package ledger

import "testing"

func TestConvert(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{"USD": 1, "IDR": 16000, "EUR": 0.5}}

	// USD -> IDR: $10.00 (1000 cents) = 160000 IDR (0 decimals).
	if got, err := r.Convert(1000, "USD", "IDR"); err != nil || got != 160000 {
		t.Errorf("USD->IDR = %d,%v want 160000,nil", got, err)
	}
	// IDR -> USD round-trips back to $10.00.
	if got, err := r.Convert(160000, "IDR", "USD"); err != nil || got != 1000 {
		t.Errorf("IDR->USD = %d,%v want 1000,nil", got, err)
	}
	// Cross rate via base: EUR -> IDR. 1 EUR = 2 USD = 32000 IDR.
	// €5.00 (500 cents) -> 160000 IDR.
	if got, err := r.Convert(500, "EUR", "IDR"); err != nil || got != 160000 {
		t.Errorf("EUR->IDR = %d,%v want 160000,nil", got, err)
	}
	// Same currency is identity.
	if got, err := r.Convert(1234, "USD", "USD"); err != nil || got != 1234 {
		t.Errorf("USD->USD = %d,%v want 1234,nil", got, err)
	}
	// Unknown currency errors.
	if _, err := r.Convert(100, "USD", "XYZ"); err == nil {
		t.Error("expected error converting to unknown currency")
	}
}

func TestRatesRoundTrip(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	in := Rates{Base: "USD", Rates: map[string]float64{"USD": 1, "IDR": 16000}}
	if err := s.SaveRates(in); err != nil {
		t.Fatal(err)
	}
	out, err := s.LoadRates()
	if err != nil {
		t.Fatal(err)
	}
	if out.Base != "USD" || out.Rates["IDR"] != 16000 {
		t.Errorf("round-trip mismatch: %+v", out)
	}
	// Missing file yields an empty, usable table.
	empty := &Store{Dir: t.TempDir()}
	if r, err := empty.LoadRates(); err != nil || r.Rates == nil {
		t.Errorf("empty LoadRates = %+v,%v", r, err)
	}
}
