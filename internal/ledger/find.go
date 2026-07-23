package ledger

import (
	"fmt"
	"sort"
	"strings"
)

// FindFilter selects transactions in Find. Zero-value fields are ignored.
type FindFilter struct {
	Text     string // case-insensitive substring of note/category/tags (incl. splits)
	Account  string // limit to one account
	Category string // exact category match (incl. split categories), case-insensitive
	Tag      string // has this tag (case-insensitive)
	Type     string // "income" | "expense" | "transfer" | ""
	Min      *Money // amount magnitude >= Min
	Max      *Money // amount magnitude <= Max
	From     string // YYYY-MM-DD inclusive
	To       string // YYYY-MM-DD inclusive
}

// Found is a transaction together with the account and month it belongs to.
type Found struct {
	Account string `json:"account"`
	Month   string `json:"month"`
	Transaction
}

// Find returns transactions across accounts/months matching f, sorted by date
// then account.
func (s *Store) Find(f FindFilter) ([]Found, error) {
	accts, err := s.LoadAccounts()
	if err != nil {
		return nil, err
	}
	var out []Found
	for _, a := range accts {
		if f.Account != "" && a.Name != f.Account {
			continue
		}
		months, err := s.Months(a.Name)
		if err != nil {
			return nil, err
		}
		for _, m := range months {
			txns, err := s.Transactions(a.Name, m)
			if err != nil {
				return nil, err
			}
			for _, t := range txns {
				if matchesFilter(t, f) {
					out = append(out, Found{Account: a.Name, Month: m, Transaction: t})
				}
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Date != out[j].Date {
			return out[i].Date < out[j].Date
		}
		return out[i].Account < out[j].Account
	})
	return out, nil
}

func matchesFilter(t Transaction, f FindFilter) bool {
	if f.From != "" && t.Date < f.From {
		return false
	}
	if f.To != "" && t.Date > f.To {
		return false
	}
	switch f.Type {
	case "transfer":
		if t.Transfer == "" {
			return false
		}
	case "income":
		if t.Transfer != "" || t.Amount < 0 {
			return false
		}
	case "expense":
		if t.Transfer != "" || t.Amount > 0 {
			return false
		}
	}
	mag := t.Amount
	if mag < 0 {
		mag = -mag
	}
	if f.Min != nil && mag < *f.Min {
		return false
	}
	if f.Max != nil && mag > *f.Max {
		return false
	}
	if f.Category != "" && !hasCategory(t, f.Category) {
		return false
	}
	if f.Tag != "" && !hasTag(t, f.Tag) {
		return false
	}
	if f.Text != "" && !matchesText(t, f.Text) {
		return false
	}
	return true
}

func hasTag(t Transaction, tag string) bool {
	tag = strings.ToLower(tag)
	for _, x := range t.Tags {
		if strings.ToLower(x) == tag {
			return true
		}
	}
	return false
}

func hasCategory(t Transaction, cat string) bool {
	cat = strings.ToLower(cat)
	if strings.ToLower(t.Category) == cat {
		return true
	}
	for _, s := range t.Splits {
		if strings.ToLower(s.Category) == cat {
			return true
		}
	}
	return false
}

func matchesText(t Transaction, q string) bool {
	q = strings.ToLower(q)
	if strings.Contains(strings.ToLower(t.Note), q) || strings.Contains(strings.ToLower(t.Category), q) {
		return true
	}
	for _, tag := range t.Tags {
		if strings.Contains(strings.ToLower(tag), q) {
			return true
		}
	}
	for _, s := range t.Splits {
		if strings.Contains(strings.ToLower(s.Category), q) || strings.Contains(strings.ToLower(s.Note), q) {
			return true
		}
	}
	return false
}

// FindTransaction locates a transaction by id, returning its account and month.
func (s *Store) FindTransaction(id string) (account, month string, t Transaction, err error) {
	accts, err := s.LoadAccounts()
	if err != nil {
		return "", "", Transaction{}, err
	}
	for _, a := range accts {
		months, err := s.Months(a.Name)
		if err != nil {
			return "", "", Transaction{}, err
		}
		for _, m := range months {
			txns, err := s.Transactions(a.Name, m)
			if err != nil {
				return "", "", Transaction{}, err
			}
			for _, tx := range txns {
				if tx.ID == id {
					return a.Name, m, tx, nil
				}
			}
		}
	}
	return "", "", Transaction{}, fmt.Errorf("transaction %q not found", id)
}

// SetAttachment records a receipt path (relative to the data dir) on a
// transaction. It does not affect balances, so no recompute is needed.
func (s *Store) SetAttachment(account, month, id, path string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	mf, err := s.LoadMonth(account, month)
	if err != nil {
		return err
	}
	for i := range mf.Transactions {
		if mf.Transactions[i].ID == id {
			mf.Transactions[i].Attachment = path
			return writeJSON(s.monthPath(account, month), mf)
		}
	}
	return fmt.Errorf("transaction %q not found in %s", id, month)
}
