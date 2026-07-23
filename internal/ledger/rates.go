package ledger

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
)

// Rates is a manual exchange-rate table. Rates[code] is how many units of `code`
// equal one unit of Base (e.g. Base "USD", Rates["IDR"]=16000 → 1 USD = 16000 IDR).
type Rates struct {
	Base  string             `json:"base"`
	Rates map[string]float64 `json:"rates"`
}

func (r Rates) rate(code string) (float64, bool) {
	if code == r.Base {
		return 1, true
	}
	v, ok := r.Rates[code]
	return v, ok
}

// Convert converts amount (minor units of `from`) into minor units of `to`,
// rounding to the target currency's smallest unit. FX conversion is inherently
// approximate; the result is for display, never stored as a transaction.
func (r Rates) Convert(amount Money, from, to string) (Money, error) {
	if from == to {
		return amount, nil
	}
	rf, ok := r.rate(from)
	if !ok || rf == 0 {
		return 0, fmt.Errorf("no exchange rate for %s", from)
	}
	rt, ok := r.rate(to)
	if !ok || rt == 0 {
		return 0, fmt.Errorf("no exchange rate for %s", to)
	}
	valFrom := float64(amount) / math.Pow10(CurrencyDecimals(from)) // decimal value in `from`
	valTo := (valFrom / rf) * rt                                    // via base currency
	return Money(int64(math.Round(valTo * math.Pow10(CurrencyDecimals(to))))), nil
}

func (s *Store) ratesPath() string { return filepath.Join(s.Dir, "rates.json") }

// LoadRates returns the rate table, or an empty one if none exists yet.
func (s *Store) LoadRates() (Rates, error) {
	var r Rates
	err := readJSON(s.ratesPath(), &r)
	if errors.Is(err, os.ErrNotExist) {
		return Rates{Rates: map[string]float64{}}, nil
	}
	if r.Rates == nil {
		r.Rates = map[string]float64{}
	}
	return r, err
}

func (s *Store) SaveRates(r Rates) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	return writeJSON(s.ratesPath(), r)
}
