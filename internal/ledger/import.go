package ledger

import "fmt"

// ImportTransactions bulk-inserts txns into account: it validates each date,
// assigns an ID where empty, groups rows by month and appends them to the
// existing month files, then recomputes once. It returns the number inserted.
//
// ponytail: a single recompute for the whole batch, vs. one per row if you
// looped AddTransaction (O(n) full-ledger recomputes). Upgrade path: none
// needed — bulk import is the whole point.
func (s *Store) ImportTransactions(account string, txns []Transaction) (int, error) {
	for i, t := range txns {
		if !validDate(t.Date) {
			return 0, fmt.Errorf("row %d: invalid date %q (want YYYY-MM-DD)", i+1, t.Date)
		}
	}
	unlock, err := s.lock()
	if err != nil {
		return 0, err
	}
	defer unlock()

	byMonth := map[string][]Transaction{}
	for _, t := range txns {
		if t.ID == "" {
			t.ID = newID()
		}
		month := t.Date[:7]
		byMonth[month] = append(byMonth[month], t)
	}
	for month, rows := range byMonth {
		mf, err := s.LoadMonth(account, month)
		if err != nil {
			return 0, err
		}
		mf.Transactions = append(mf.Transactions, rows...)
		if err := writeJSON(s.monthPath(account, month), mf); err != nil {
			return 0, err
		}
	}
	if _, err := s.recompute(account); err != nil {
		return 0, err
	}
	return len(txns), nil
}
