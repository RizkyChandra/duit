package ledger

import "testing"

func TestImportTransactions(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Fatal(err)
	}
	txns := []Transaction{
		{Date: "2026-01-05", Amount: 10000, Category: "salary"},
		{Date: "2026-01-20", Amount: -2500, Category: "food"},
		{Date: "2026-02-03", Amount: -1500, Category: "food"},
	}
	n, err := s.ImportTransactions("cash", txns)
	if err != nil {
		t.Fatal(err)
	}
	if n != 3 {
		t.Fatalf("imported %d want 3", n)
	}

	// Rows land in the right month files.
	if jan, _ := s.Transactions("cash", "2026-01"); len(jan) != 2 {
		t.Errorf("Jan has %d txns want 2", len(jan))
	}
	if feb, _ := s.Transactions("cash", "2026-02"); len(feb) != 1 {
		t.Errorf("Feb has %d txns want 1", len(feb))
	}
	// Every imported row got an ID.
	for _, m := range []string{"2026-01", "2026-02"} {
		rows, _ := s.Transactions("cash", m)
		for _, r := range rows {
			if r.ID == "" {
				t.Errorf("txn %+v missing ID", r)
			}
		}
	}
	// Cached balance == sum of amounts (10000 - 2500 - 1500 = 6000).
	if acct, ok, _ := s.Account("cash"); !ok || acct.Balance != 6000 {
		t.Errorf("balance = %d want 6000", acct.Balance)
	}
}

func TestImportRejectsBadDate(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Fatal(err)
	}
	_, err := s.ImportTransactions("cash", []Transaction{{Date: "not-a-date", Amount: 1}})
	if err == nil {
		t.Fatal("expected error on bad date")
	}
}
