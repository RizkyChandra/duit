package ledger

import "testing"

func TestRenameCategoryMigration(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	must(t, s.AddAccount(Account{Name: "bank", Currency: "USD", Decimals: 2}))
	must(t, s.AddCategory("food"))
	must(t, s.SetBudget("food", 10000))

	// Plain txn (May, cash), split txn (June, bank), and a non-matching one.
	if _, err := s.AddTransaction("cash", Transaction{Date: "2026-05-02", Amount: -1200, Category: "food"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTransaction("bank", Transaction{Date: "2026-06-03", Amount: -3000, Splits: []Split{
		{Category: "food", Amount: -2000}, {Category: "misc", Amount: -1000},
	}}); err != nil {
		t.Fatal(err)
	}

	n, err := s.RenameCategory("food", "groceries")
	must(t, err)
	if n != 1 {
		t.Errorf("changed top-level = %d want 1", n) // 1 plain txn; the split is a split, not a top-level match
	}

	// Registry updated.
	cats, _ := s.LoadCategories()
	if contains(cats, "food") || !contains(cats, "groceries") {
		t.Errorf("registry = %v want groceries not food", cats)
	}
	// Plain txn migrated.
	if txns, _ := s.Transactions("cash", "2026-05"); txns[0].Category != "groceries" {
		t.Errorf("plain category = %q want groceries", txns[0].Category)
	}
	// Split migrated (only the food split; misc untouched).
	txns, _ := s.Transactions("bank", "2026-06")
	sp := txns[0].Splits
	if sp[0].Category != "groceries" || sp[1].Category != "misc" {
		t.Errorf("split categories = %q/%q want groceries/misc", sp[0].Category, sp[1].Category)
	}
	// Budget migrated.
	lines, _ := s.BudgetStatus("2026-05")
	found := false
	for _, l := range lines {
		if l.Category == "groceries" {
			found = true
		}
		if l.Category == "food" {
			t.Error("old budget category still present")
		}
	}
	if !found {
		t.Error("budget not migrated to groceries")
	}
}

func contains(ss []string, x string) bool {
	for _, s := range ss {
		if s == x {
			return true
		}
	}
	return false
}
