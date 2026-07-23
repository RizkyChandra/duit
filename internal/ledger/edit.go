package ledger

import "fmt"

// EditTransaction replaces the fields of the transaction with updated.ID,
// preserving its ID. If the date's month changed it moves the entry to the new
// month file. Splits are validated and the account's balance recomputed.
func (s *Store) EditTransaction(updated Transaction) error {
	if updated.ID == "" {
		return fmt.Errorf("edit requires a transaction id")
	}
	if !validDate(updated.Date) {
		return fmt.Errorf("invalid date %q (want YYYY-MM-DD)", updated.Date)
	}
	if err := validateSplits(updated); err != nil {
		return err
	}
	account, oldMonth, _, err := s.FindTransaction(updated.ID)
	if err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()

	newMonth := updated.Date[:7]
	if newMonth == oldMonth {
		mf, err := s.LoadMonth(account, oldMonth)
		if err != nil {
			return err
		}
		for i := range mf.Transactions {
			if mf.Transactions[i].ID == updated.ID {
				mf.Transactions[i] = updated
				break
			}
		}
		if err := writeJSON(s.monthPath(account, oldMonth), mf); err != nil {
			return err
		}
	} else {
		oldMf, err := s.LoadMonth(account, oldMonth)
		if err != nil {
			return err
		}
		kept := oldMf.Transactions[:0:0]
		for _, t := range oldMf.Transactions {
			if t.ID != updated.ID {
				kept = append(kept, t)
			}
		}
		oldMf.Transactions = kept
		if err := writeJSON(s.monthPath(account, oldMonth), oldMf); err != nil {
			return err
		}
		newMf, err := s.LoadMonth(account, newMonth)
		if err != nil {
			return err
		}
		newMf.Transactions = append(newMf.Transactions, updated)
		if err := writeJSON(s.monthPath(account, newMonth), newMf); err != nil {
			return err
		}
	}
	_, err = s.recompute(account)
	return err
}
