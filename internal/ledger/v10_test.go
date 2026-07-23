package ledger

import (
	"math"
	"testing"
	"time"
)

func TestParseMoneyRejectsGarbage(t *testing.T) {
	for _, s := range []string{"-", "+", ".", "+-5", "-+5", "5-", "1.-2", "- 5", "1 000"} {
		if v, err := ParseMoney(s, 2); err == nil {
			t.Errorf("ParseMoney(%q) accepted, got %d", s, v)
		}
	}
	// Sanity: valid ones still parse.
	for _, s := range []string{".5", "12.", "-5", "+7.08", "0"} {
		if _, err := ParseMoney(s, 2); err != nil {
			t.Errorf("ParseMoney(%q) rejected: %v", s, err)
		}
	}
}

func TestFormatMinInt64(t *testing.T) {
	got := Money(math.MinInt64).Format(2) // must not panic or produce garbage
	if got != "-92233720368547758.08" {
		t.Errorf("MinInt64.Format(2) = %q", got)
	}
	if Money(math.MaxInt64).Format(0) != "9223372036854775807" {
		t.Errorf("MaxInt64.Format(0) wrong")
	}
}

func TestConvertZeroRate(t *testing.T) {
	r := Rates{Base: "USD", Rates: map[string]float64{"USD": 1, "IDR": 0}}
	if _, err := r.Convert(100, "USD", "IDR"); err == nil {
		t.Error("Convert with a zero destination rate should error, not return 0")
	}
}

func TestValidAccountNameRejectsTraversal(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	for _, bad := range []string{"../x", "a/b", "..", ".", `a\b`} {
		if err := s.AddAccount(Account{Name: bad, Currency: "USD", Decimals: 2}); err == nil {
			t.Errorf("AddAccount accepted unsafe name %q", bad)
		}
	}
	if err := s.AddAccount(Account{Name: "my cash", Currency: "USD", Decimals: 2}); err != nil {
		t.Errorf("AddAccount rejected a safe name: %v", err)
	}
}

func TestRecurringNotBeforeStart(t *testing.T) {
	// Monthly on day 15 starting the 20th: the first occurrence must be the NEXT
	// month's 15th, never the 15th of the start month (which precedes Start).
	r := Recurring{Cadence: "monthly", Interval: 1, Day: 15, Start: "2026-01-20"}
	until, _ := time.Parse(dateLayout, "2026-03-31")
	got := r.dueDates(until)
	want := []string{"2026-02-15", "2026-03-15"}
	if len(got) != len(want) || got[0] != want[0] {
		t.Errorf("dueDates = %v want %v", got, want)
	}
}

func TestRecurringIdempotentAfterFailure(t *testing.T) {
	s := &Store{Dir: t.TempDir()}
	must(t, s.AddAccount(Account{Name: "usd", Currency: "USD", Decimals: 2}))
	must(t, s.AddAccount(Account{Name: "idr", Currency: "IDR", Decimals: 0}))
	// Rule A: plain income (will succeed). Rule B: cross-currency transfer with no
	// FX rate set (will fail). Both due 2026-01-01.
	_, err := s.AddRecurring(Recurring{Account: "usd", Amount: 1000, Cadence: "monthly", Day: 1, Start: "2026-01-01"})
	must(t, err)
	_, err = s.AddRecurring(Recurring{Account: "usd", To: "idr", Amount: 500, Cadence: "monthly", Day: 1, Start: "2026-01-01"})
	must(t, err)

	if n, err := s.ApplyRecurring("2026-01-15"); err == nil || n != 1 {
		t.Fatalf("first apply = (%d,%v), want (1, error from the failed transfer)", n, err)
	}
	// Retry: rule A is already applied and must not be duplicated.
	if n, err := s.ApplyRecurring("2026-01-15"); err == nil || n != 0 {
		t.Fatalf("retry = (%d,%v), want (0, error) — rule A must not replay", n, err)
	}
	if a, _, _ := s.Account("usd"); a.Balance != 1000 {
		t.Errorf("usd balance = %d want 1000 (income recorded exactly once)", a.Balance)
	}
}
