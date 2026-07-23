package ledger

import "testing"

// FuzzParseMoney checks that ParseMoney never panics and that any value it
// accepts survives a Format→ParseMoney round-trip unchanged (no money drift).
func FuzzParseMoney(f *testing.F) {
	for _, s := range []string{"12.34", "-5", "0", "1000", ".5", "12.", "+7.08", "abc", "", "1.2.3", "-.0", "000.10"} {
		f.Add(s, 2)
		f.Add(s, 0)
	}
	f.Fuzz(func(t *testing.T, s string, decimals int) {
		if decimals < 0 || decimals > 8 {
			return // realistic currency range; Format is undefined outside it
		}
		m, err := ParseMoney(s, decimals)
		if err != nil {
			return // rejected inputs are fine
		}
		formatted := m.Format(decimals)
		m2, err := ParseMoney(formatted, decimals)
		if err != nil {
			t.Fatalf("re-parsing formatted %q (from %q, dec=%d) failed: %v", formatted, s, decimals, err)
		}
		if m2 != m {
			t.Fatalf("round-trip drift: %q dec=%d -> %d -> %q -> %d", s, decimals, m, formatted, m2)
		}
	})
}
