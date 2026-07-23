package tui

import (
	"strings"
	"testing"

	"github.com/RizkyChandra/duit/internal/ledger"

	tea "github.com/charmbracelet/bubbletea"
)

func seedStore(t *testing.T) *ledger.Store {
	t.Helper()
	s := &ledger.Store{Dir: t.TempDir()}
	if err := s.AddAccount(ledger.Account{Name: "wallet", Currency: "USD", Decimals: 2, Created: "2026-01-01"}); err != nil {
		t.Fatalf("AddAccount: %v", err)
	}
	for _, tx := range []ledger.Transaction{
		{Date: "2026-07-10", Amount: -1250, Category: "food", Note: "lunch"},
		{Date: "2026-07-11", Amount: 50000, Category: "salary"},
	} {
		if _, err := s.AddTransaction("wallet", tx); err != nil {
			t.Fatalf("AddTransaction: %v", err)
		}
	}
	return s
}

func TestAccountScreenRenders(t *testing.T) {
	m := New(seedStore(t), nil)
	v := m.View()
	if v == "" {
		t.Fatal("empty view")
	}
	if !strings.Contains(v, "wallet") {
		t.Fatalf("account name not in view:\n%s", v)
	}
}

func TestEnterOpensTransactions(t *testing.T) {
	m := New(seedStore(t), nil)
	next, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	rm := next.(Model)
	if rm.screen != screenTxns {
		t.Fatalf("expected txn screen, got %d", rm.screen)
	}
	v := rm.View()
	// Header carries the account name + balance; rows carry categories.
	for _, want := range []string{"wallet", "food", "salary"} {
		if !strings.Contains(v, want) {
			t.Fatalf("txn view missing %q:\n%s", want, v)
		}
	}
}

func TestAddTransactionFlow(t *testing.T) {
	s := seedStore(t)
	m := New(s, nil)
	drive := func(mod tea.Model, msg tea.Msg) Model {
		n, _ := mod.Update(msg)
		return n.(Model)
	}
	cur := drive(m, tea.KeyMsg{Type: tea.KeyEnter})                      // open account
	cur = drive(cur, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}}) // add form
	if cur.screen != screenForm {
		t.Fatalf("expected form screen")
	}
	// amount field is focused; type an amount, then jump to the date field
	// (default today) and submit.
	for _, r := range "-3.25" {
		cur = drive(cur, tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{r}})
	}
	cur = drive(cur, tea.KeyMsg{Type: tea.KeyEnter}) // submit
	if cur.err != nil {
		t.Fatalf("submit error: %v", cur.err)
	}
	if cur.screen != screenTxns {
		t.Fatalf("expected return to txn screen, got %d", cur.screen)
	}
	// New txn persisted: -325 minor units should exist in this month.
	txns, err := s.Transactions("wallet", cur.month)
	if err != nil {
		t.Fatalf("Transactions: %v", err)
	}
	found := false
	for _, tx := range txns {
		if tx.Amount == -325 {
			found = true
		}
	}
	if !found {
		t.Fatalf("added transaction not persisted: %+v", txns)
	}
}
