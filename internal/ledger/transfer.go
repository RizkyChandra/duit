package ledger

import "fmt"

// Transfer moves `amount` (minor units of the source account's currency) from
// `from` to `to` as a linked pair of transactions. For cross-currency transfers
// the destination amount is the fx-rate conversion of `amount`, unless
// destOverride is non-nil (the exact figure the destination actually received).
// Both legs carry a Transfer field naming the counterparty so they are excluded
// from income/expense aggregation. Returns the (source, dest) amounts recorded.
func (s *Store) Transfer(from, to string, amount Money, destOverride *Money, date, note string) (Money, Money, error) {
	if from == to {
		return 0, 0, fmt.Errorf("cannot transfer to the same account")
	}
	if amount <= 0 {
		return 0, 0, fmt.Errorf("transfer amount must be positive")
	}
	if !validDate(date) {
		return 0, 0, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", date)
	}
	src, ok, err := s.Account(from)
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, fmt.Errorf("unknown account %q", from)
	}
	dst, ok, err := s.Account(to)
	if err != nil {
		return 0, 0, err
	}
	if !ok {
		return 0, 0, fmt.Errorf("unknown account %q", to)
	}

	// Determine the destination amount.
	var destAmt Money
	switch {
	case destOverride != nil:
		destAmt = *destOverride
	case src.Currency == dst.Currency:
		destAmt = amount
	default:
		rates, err := s.LoadRates()
		if err != nil {
			return 0, 0, err
		}
		destAmt, err = rates.Convert(amount, src.Currency, dst.Currency)
		if err != nil {
			return 0, 0, fmt.Errorf("%w — set a rate with `duit fx set` or pass --dest-amount", err)
		}
	}

	unlock, err := s.lock()
	if err != nil {
		return 0, 0, err
	}
	defer unlock()

	out := Transaction{ID: newID(), Date: date, Amount: -amount, Category: "Transfer", Note: note, Transfer: to}
	in := Transaction{ID: newID(), Date: date, Amount: destAmt, Category: "Transfer", Note: note, Transfer: from}
	if err := s.appendLocked(from, out); err != nil {
		return 0, 0, err
	}
	if err := s.appendLocked(to, in); err != nil {
		return 0, 0, err
	}
	if _, err := s.recompute(from); err != nil {
		return 0, 0, err
	}
	if _, err := s.recompute(to); err != nil {
		return 0, 0, err
	}
	return amount, destAmt, nil
}

// appendLocked appends a transaction to its month file. The caller must hold the
// lock and is responsible for recomputing afterwards.
func (s *Store) appendLocked(account string, t Transaction) error {
	month := t.Date[:7]
	mf, err := s.LoadMonth(account, month)
	if err != nil {
		return err
	}
	mf.Transactions = append(mf.Transactions, t)
	return writeJSON(s.monthPath(account, month), mf)
}
