package ledger

import "testing"

func TestAddAndRecompute(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Fatal(err)
	}
	add := func(date string, amt Money) {
		t.Helper()
		if _, err := s.AddTransaction("cash", Transaction{Date: date, Amount: amt}); err != nil {
			t.Fatal(err)
		}
	}
	add("2026-01-05", 10000) // +100.00
	add("2026-02-10", -3000) // -30.00
	add("2026-01-20", 500)   // backdated +5.00 into January

	// January: opening 0, closing 105.00 (backdated txn included).
	if jan, _ := s.LoadMonth("cash", "2026-01"); jan.Opening != 0 || jan.Closing != 10500 {
		t.Errorf("Jan opening/closing = %d/%d want 0/10500", jan.Opening, jan.Closing)
	}
	// February: opening = Jan closing, closing 75.00.
	if feb, _ := s.LoadMonth("cash", "2026-02"); feb.Opening != 10500 || feb.Closing != 7500 {
		t.Errorf("Feb opening/closing = %d/%d want 10500/7500", feb.Opening, feb.Closing)
	}
	// Cached balance reflects the running total.
	if acct, ok, _ := s.Account("cash"); !ok || acct.Balance != 7500 {
		t.Errorf("balance = %d want 7500", acct.Balance)
	}
	// Recompute is idempotent (also proves the lock was released each call).
	if bal, err := s.Recompute("cash"); err != nil || bal != 7500 {
		t.Errorf("Recompute = %d,%v want 7500,nil", bal, err)
	}
}

func TestRemoveTransaction(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Fatal(err)
	}
	kept, err := s.AddTransaction("cash", Transaction{Date: "2026-03-01", Amount: 1000})
	if err != nil {
		t.Fatal(err)
	}
	drop, err := s.AddTransaction("cash", Transaction{Date: "2026-03-02", Amount: 400})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveTransaction("cash", "2026-03", drop.ID); err != nil {
		t.Fatal(err)
	}
	txns, _ := s.Transactions("cash", "2026-03")
	if len(txns) != 1 || txns[0].ID != kept.ID {
		t.Errorf("after remove, txns = %+v want only %s", txns, kept.ID)
	}
	if acct, _, _ := s.Account("cash"); acct.Balance != 1000 {
		t.Errorf("balance = %d want 1000", acct.Balance)
	}
	if err := s.RemoveTransaction("cash", "2026-03", "nope"); err == nil {
		t.Error("removing missing id should error")
	}
}
