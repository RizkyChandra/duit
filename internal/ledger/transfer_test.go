package ledger

import "testing"

func TestTransferSameCurrency(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}))
	must(t, s.AddAccount(Account{Name: "bank", Currency: "USD", Decimals: 2}))
	if _, err := s.AddTransaction("cash", Transaction{Date: "2026-05-01", Amount: 10000}); err != nil {
		t.Fatal(err)
	}
	src, dst, err := s.Transfer("cash", "bank", 3000, nil, "2026-05-02", "rent stash")
	if err != nil {
		t.Fatal(err)
	}
	if src != 3000 || dst != 3000 {
		t.Errorf("amounts = %d/%d want 3000/3000", src, dst)
	}
	if a, _, _ := s.Account("cash"); a.Balance != 7000 {
		t.Errorf("cash = %d want 7000", a.Balance)
	}
	if a, _, _ := s.Account("bank"); a.Balance != 3000 {
		t.Errorf("bank = %d want 3000", a.Balance)
	}
}

func TestTransferCrossCurrency(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "usd", Currency: "USD", Decimals: 2}))
	must(t, s.AddAccount(Account{Name: "idr", Currency: "IDR", Decimals: 0}))
	must(t, s.SaveRates(Rates{Base: "USD", Rates: map[string]float64{"USD": 1, "IDR": 16000}}))
	if _, err := s.AddTransaction("usd", Transaction{Date: "2026-05-01", Amount: 100000}); err != nil {
		t.Fatal(err)
	}
	// Auto-convert: $10.00 -> 160000 IDR.
	_, dst, err := s.Transfer("usd", "idr", 1000, nil, "2026-05-02", "")
	if err != nil {
		t.Fatal(err)
	}
	if dst != 160000 {
		t.Errorf("auto dst = %d want 160000", dst)
	}
	// Override: bank actually credited 158000 IDR (fees/real rate).
	override := Money(158000)
	_, dst2, err := s.Transfer("usd", "idr", 1000, &override, "2026-05-03", "")
	if err != nil {
		t.Fatal(err)
	}
	if dst2 != 158000 {
		t.Errorf("override dst = %d want 158000", dst2)
	}
	if a, _, _ := s.Account("idr"); a.Balance != 318000 {
		t.Errorf("idr balance = %d want 318000", a.Balance)
	}
}

func TestTransferExcludedFromBudget(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "a", Currency: "USD", Decimals: 2}))
	must(t, s.AddAccount(Account{Name: "b", Currency: "USD", Decimals: 2}))
	if _, err := s.AddTransaction("a", Transaction{Date: "2026-05-01", Amount: 10000}); err != nil {
		t.Fatal(err)
	}
	must(t, s.SetBudget("Transfer", 100)) // even with a budget named Transfer...
	if _, _, err := s.Transfer("a", "b", 5000, nil, "2026-05-02", ""); err != nil {
		t.Fatal(err)
	}
	lines, err := s.BudgetStatus("2026-05")
	if err != nil {
		t.Fatal(err)
	}
	for _, l := range lines {
		if l.Category == "Transfer" && l.Spent != 0 {
			t.Errorf("transfer leg counted as spend: %d", l.Spent)
		}
	}
}

func must(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatal(err)
	}
}
