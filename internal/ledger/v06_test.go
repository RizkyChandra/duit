package ledger

import "testing"

func TestSplitValidation(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	// Splits must sum to the amount.
	bad := Transaction{Date: "2026-05-01", Amount: -7000, Splits: []Split{
		{Category: "food", Amount: -5000}, {Category: "household", Amount: -1000},
	}}
	if _, err := s.AddTransaction("cash", bad); err == nil {
		t.Error("expected error when splits do not sum to amount")
	}
	good := Transaction{Date: "2026-05-01", Amount: -7000, Splits: []Split{
		{Category: "food", Amount: -5000}, {Category: "household", Amount: -2000},
	}}
	if _, err := s.AddTransaction("cash", good); err != nil {
		t.Fatalf("valid split rejected: %v", err)
	}
	// Balance uses the total, unaffected by the split breakdown.
	if a, _, _ := s.Account("cash"); a.Balance != -7000 {
		t.Errorf("balance = %d want -7000", a.Balance)
	}
}

func TestBudgetSplitAttribution(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	must(t, s.SetBudget("food", 10000))
	must(t, s.SetBudget("household", 10000))
	_, err := s.AddTransaction("cash", Transaction{Date: "2026-05-01", Amount: -7000, Splits: []Split{
		{Category: "food", Amount: -5000}, {Category: "household", Amount: -2000},
	}})
	must(t, err)
	lines, err := s.BudgetStatus("2026-05")
	must(t, err)
	got := map[string]Money{}
	for _, l := range lines {
		got[l.Category] = l.Spent
	}
	if got["food"] != 5000 || got["household"] != 2000 {
		t.Errorf("split attribution = food %d household %d want 5000/2000", got["food"], got["household"])
	}
}

func TestFindAndAttachment(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	must(t, s.AddAccount(Account{Name: "bank", Currency: "USD", Decimals: 2}))
	pay, _ := s.AddTransaction("cash", Transaction{Date: "2026-05-02", Amount: 3000, Category: "salary", Note: "May pay"})
	s.AddTransaction("cash", Transaction{Date: "2026-05-05", Amount: -1200, Category: "food", Note: "dinner"})
	s.AddTransaction("bank", Transaction{Date: "2026-06-01", Amount: -900, Category: "food", Note: "lunch"})

	// Text search.
	if got, _ := s.Find(FindFilter{Text: "lunch"}); len(got) != 1 || got[0].Account != "bank" {
		t.Errorf("text find = %+v", got)
	}
	// Category + type.
	if got, _ := s.Find(FindFilter{Category: "food", Type: "expense"}); len(got) != 2 {
		t.Errorf("food expenses = %d want 2", len(got))
	}
	// Amount magnitude range.
	min := Money(1000)
	if got, _ := s.Find(FindFilter{Min: &min}); len(got) != 2 {
		t.Errorf("min 1000 = %d want 2 (3000 and 1200)", len(got))
	}
	// Date range.
	if got, _ := s.Find(FindFilter{From: "2026-06-01"}); len(got) != 1 {
		t.Errorf("from June = %d want 1", len(got))
	}

	// Attachment.
	acct, month, _, err := s.FindTransaction(pay.ID)
	must(t, err)
	must(t, s.SetAttachment(acct, month, pay.ID, "attachments/cash/x.jpg"))
	_, _, tx, _ := s.FindTransaction(pay.ID)
	if tx.Attachment != "attachments/cash/x.jpg" {
		t.Errorf("attachment = %q", tx.Attachment)
	}
}
