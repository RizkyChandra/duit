package ledger

import "testing"

func TestApplyRecurringMonthly(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRecurring(Recurring{
		Account: "cash", Amount: -1500, Category: "rent",
		Cadence: "monthly", Interval: 1, Day: 15, Start: "2026-01-15",
	}); err != nil {
		t.Fatal(err)
	}

	n, err := s.ApplyRecurring("2026-04-20")
	if err != nil {
		t.Fatal(err)
	}
	if n != 4 { // Jan/Feb/Mar/Apr 15
		t.Fatalf("first apply created %d, want 4", n)
	}
	// Idempotent: same until creates nothing.
	if n, err := s.ApplyRecurring("2026-04-20"); err != nil || n != 0 {
		t.Fatalf("second apply = %d,%v want 0,nil", n, err)
	}
	// Verify the materialized transactions land on the 15th of each month.
	for _, m := range []string{"2026-01", "2026-02", "2026-03", "2026-04"} {
		txns, _ := s.Transactions("cash", m)
		if len(txns) != 1 || txns[0].Date != m+"-15" || txns[0].Amount != -1500 {
			t.Errorf("%s: %+v, want one txn on %s-15 for -1500", m, txns, m)
		}
	}
}

func TestApplyRecurringDaily(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRecurring(Recurring{
		Account: "cash", Amount: -100,
		Cadence: "daily", Interval: 2, Start: "2026-01-01",
	}); err != nil {
		t.Fatal(err)
	}
	// Jan 1,3,5,7,9 -> 5 occurrences up to Jan 10.
	if n, err := s.ApplyRecurring("2026-01-10"); err != nil || n != 5 {
		t.Fatalf("daily apply = %d,%v want 5,nil", n, err)
	}
	if n, _ := s.ApplyRecurring("2026-01-10"); n != 0 {
		t.Errorf("daily re-apply created %d, want 0", n)
	}
	// Extending the window continues from where it left off (Jan 11).
	if n, _ := s.ApplyRecurring("2026-01-12"); n != 1 {
		t.Errorf("daily extend created %d, want 1 (Jan 11)", n)
	}
	txns, _ := s.Transactions("cash", "2026-01")
	if len(txns) != 6 {
		t.Errorf("got %d txns, want 6", len(txns))
	}
}

func TestApplyRecurringMonthlyClamp(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	if err := s.AddAccount(Account{Name: "cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddRecurring(Recurring{
		Account: "cash", Amount: -900,
		Cadence: "monthly", Interval: 1, Day: 31, Start: "2026-01-31",
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyRecurring("2026-03-31"); err != nil {
		t.Fatal(err)
	}
	// 2026 is not a leap year: Feb clamps to the 28th.
	if txns, _ := s.Transactions("cash", "2026-02"); len(txns) != 1 || txns[0].Date != "2026-02-28" {
		t.Errorf("Feb: %+v, want one txn on 2026-02-28", txns)
	}
	if txns, _ := s.Transactions("cash", "2026-01"); len(txns) != 1 || txns[0].Date != "2026-01-31" {
		t.Errorf("Jan: %+v, want one txn on 2026-01-31", txns)
	}
	if txns, _ := s.Transactions("cash", "2026-03"); len(txns) != 1 || txns[0].Date != "2026-03-31" {
		t.Errorf("Mar: %+v, want one txn on 2026-03-31", txns)
	}
}
