package ledger

import "testing"

func TestBudgetStatus(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Fatal(err)
	}
	add := func(date string, amt Money, cat string) {
		t.Helper()
		if _, err := s.AddTransaction("cash", Transaction{Date: date, Amount: amt, Category: cat}); err != nil {
			t.Fatal(err)
		}
	}
	add("2026-03-02", -3000, "food") // -30.00
	add("2026-03-15", -2500, "food") // -25.00
	add("2026-03-20", 5000, "food")  // income in same category, must be ignored
	add("2026-03-10", -1000, "rent") // different category, not budgeted

	if err := s.SetBudget("food", 10000); err != nil { // 100.00
		t.Fatal(err)
	}

	lines, err := s.BudgetStatus("2026-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	l := lines[0]
	if l.Category != "food" || l.Spent != 5500 || l.Remaining != 4500 || l.Over {
		t.Errorf("line = %+v, want food spent=5500 remaining=4500 over=false", l)
	}

	// Lower the limit below spend -> Over.
	if err := s.SetBudget("food", 5000); err != nil { // upsert, not duplicate
		t.Fatal(err)
	}
	if bs, _ := s.LoadBudgets(); len(bs) != 1 {
		t.Fatalf("upsert produced %d budgets, want 1", len(bs))
	}
	lines, _ = s.BudgetStatus("2026-03")
	if l := lines[0]; l.Spent != 5500 || l.Remaining != -500 || !l.Over {
		t.Errorf("line = %+v, want spent=5500 remaining=-500 over=true", l)
	}

	if err := s.RemoveBudget("nope"); err == nil {
		t.Error("RemoveBudget of missing category should error")
	}
	if err := s.RemoveBudget("food"); err != nil {
		t.Fatalf("RemoveBudget(food): %v", err)
	}
}
