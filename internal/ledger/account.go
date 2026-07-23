package ledger

import (
	"fmt"
	"strings"
)

// Account is a tracked account with a cached running balance. The balance is a
// cache; the authoritative source is the per-month transaction files.
type Account struct {
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Decimals int    `json:"decimals"`
	Type     string `json:"type,omitempty"` // e.g. cash, bank, credit
	Balance  Money  `json:"balance"`
	Created  string `json:"created"`            // YYYY-MM-DD
	Archived bool   `json:"archived,omitempty"` // hidden from default views; data retained
}

// SetArchived marks an account archived (hidden from default lists) or active.
func (s *Store) SetArchived(name string, archived bool) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	accts, err := s.LoadAccounts()
	if err != nil {
		return err
	}
	for i := range accts {
		if accts[i].Name == name {
			accts[i].Archived = archived
			return s.SaveAccounts(accts)
		}
	}
	return fmt.Errorf("account %q not found", name)
}

// zeroDecimalCurrencies are currencies conventionally written without minor
// units. Everything else defaults to 2. IDR is officially 2 but written as 0
// in practice, which is how people actually enter it.
var zeroDecimalCurrencies = map[string]bool{
	"JPY": true, "KRW": true, "IDR": true, "VND": true,
	"CLP": true, "ISK": true, "HUF": true, "XOF": true, "XAF": true,
}

// CurrencyDecimals returns the conventional number of fractional digits for a
// currency code, defaulting to 2.
func CurrencyDecimals(code string) int {
	if zeroDecimalCurrencies[strings.ToUpper(code)] {
		return 0
	}
	return 2
}
