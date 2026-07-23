package ledger

import "testing"

func TestEditTransaction(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	tx, err := s.AddTransaction("cash", Transaction{Date: "2026-05-01", Amount: -1000, Category: "food"})
	must(t, err)

	// Edit amount in place.
	tx.Amount = -1500
	must(t, s.EditTransaction(tx))
	if a, _, _ := s.Account("cash"); a.Balance != -1500 {
		t.Errorf("balance after amount edit = %d want -1500", a.Balance)
	}

	// Edit date into a different month: entry moves, id preserved.
	tx.Date = "2026-06-02"
	must(t, s.EditTransaction(tx))
	if m, _ := s.Transactions("cash", "2026-05"); len(m) != 0 {
		t.Errorf("May still has %d txns after move", len(m))
	}
	jun, _ := s.Transactions("cash", "2026-06")
	if len(jun) != 1 || jun[0].ID != tx.ID {
		t.Errorf("June txns = %+v want the moved entry", jun)
	}
	if a, _, _ := s.Account("cash"); a.Balance != -1500 {
		t.Errorf("balance after date edit = %d want -1500", a.Balance)
	}
}

func TestVerify(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	_, err := s.AddTransaction("cash", Transaction{Date: "2026-05-01", Amount: -1000})
	must(t, err)
	if issues, _ := s.Verify(); len(issues) != 0 {
		t.Errorf("clean ledger has issues: %+v", issues)
	}
	// Inject cached-balance drift.
	accts, _ := s.LoadAccounts()
	accts[0].Balance = 12345
	must(t, s.SaveAccounts(accts))
	issues, _ := s.Verify()
	if len(issues) == 0 || issues[0].Kind != "balance" {
		t.Errorf("expected a balance issue, got %+v", issues)
	}
	// Repair.
	_, err = s.Recompute("cash")
	must(t, err)
	if issues, _ := s.Verify(); len(issues) != 0 {
		t.Errorf("issues after recompute: %+v", issues)
	}
}

func TestFindByTag(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	s.AddTransaction("cash", Transaction{Date: "2026-05-01", Amount: -100, Tags: []string{"reimbursable"}})
	s.AddTransaction("cash", Transaction{Date: "2026-05-02", Amount: -200, Tags: []string{"vacation"}})
	if got, _ := s.Find(FindFilter{Tag: "reimbursable"}); len(got) != 1 {
		t.Errorf("tag filter = %d want 1", len(got))
	}
	if got, _ := s.Find(FindFilter{Text: "vacation"}); len(got) != 1 {
		t.Errorf("text search of tags = %d want 1", len(got))
	}
}
