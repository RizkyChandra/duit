package ledger

import "testing"

func TestRecurringTransferApply(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	must(t, s.AddAccount(Account{Name: "savings", Currency: "USD", Decimals: 2}))
	_, err := s.AddTransaction("cash", Transaction{Date: "2025-12-01", Amount: 100000})
	must(t, err)
	_, err = s.AddRecurring(Recurring{Account: "cash", To: "savings", Amount: 10000, Cadence: "monthly", Day: 1, Start: "2026-01-01"})
	must(t, err)

	n, err := s.ApplyRecurring("2026-03-15")
	must(t, err)
	if n != 3 { // Jan/Feb/Mar
		t.Errorf("applied %d want 3", n)
	}
	if a, _, _ := s.Account("cash"); a.Balance != 70000 {
		t.Errorf("cash = %d want 70000", a.Balance)
	}
	if a, _, _ := s.Account("savings"); a.Balance != 30000 {
		t.Errorf("savings = %d want 30000", a.Balance)
	}
	if n, _ := s.ApplyRecurring("2026-03-15"); n != 0 {
		t.Errorf("re-apply = %d want 0 (idempotent)", n)
	}
	// Each transfer wrote two legs, both marked as transfers.
	if found, _ := s.Find(FindFilter{Type: "transfer"}); len(found) != 6 {
		t.Errorf("transfer legs = %d want 6", len(found))
	}
}

func TestSetArchived(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "old", Currency: "USD", Decimals: 2}))
	must(t, s.SetArchived("old", true))
	if a, ok, _ := s.Account("old"); !ok || !a.Archived {
		t.Error("account not archived")
	}
	must(t, s.SetArchived("old", false))
	if a, _, _ := s.Account("old"); a.Archived {
		t.Error("account still archived after unarchive")
	}
	if err := s.SetArchived("nope", true); err == nil {
		t.Error("archiving a missing account should error")
	}
}
