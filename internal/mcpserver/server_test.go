package mcpserver

import (
	"context"
	"strings"
	"testing"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// seed builds a temp-dir Store with one account and one transaction.
func seed(t *testing.T) *ledger.Store {
	t.Helper()
	s := &ledger.Store{Dir: t.TempDir()}
	if err := s.AddAccount(ledger.Account{Name: "cash", Currency: "USD", Decimals: 2, Created: "2026-01-01"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AddTransaction("cash", ledger.Transaction{Date: "2026-07-01", Amount: 5000, Category: "salary"}); err != nil {
		t.Fatal(err)
	}
	return s
}

// connect wires an in-memory client to a server over the ledger store.
func connect(t *testing.T, store *ledger.Store) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	c1, c2 := mcp.NewInMemoryTransports()
	if _, err := newServer(store).Connect(ctx, c1, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "v0"}, nil).Connect(ctx, c2, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func call(t *testing.T, cs *mcp.ClientSession, name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: name, Arguments: args})
	if err != nil {
		t.Fatalf("%s: transport error: %v", name, err)
	}
	return res
}

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if len(res.Content) == 0 {
		t.Fatal("no content")
	}
	return res.Content[0].(*mcp.TextContent).Text
}

// TestSmoke verifies the server constructs and registers all five tools.
func TestSmoke(t *testing.T) {
	cs := connect(t, seed(t))
	var names []string
	for tool, err := range cs.Tools(context.Background(), nil) {
		if err != nil {
			t.Fatal(err)
		}
		names = append(names, tool.Name)
	}
	for _, want := range []string{"list_accounts", "get_balance", "add_transaction", "list_transactions", "summary"} {
		found := false
		for _, n := range names {
			if n == want {
				found = true
			}
		}
		if !found {
			t.Errorf("tool %q not registered; got %v", want, names)
		}
	}
}

func TestListAndBalance(t *testing.T) {
	cs := connect(t, seed(t))
	if got := textOf(t, call(t, cs, "list_accounts", nil)); !strings.Contains(got, "cash") || !strings.Contains(got, "50.00") {
		t.Errorf("list_accounts = %q", got)
	}
	if got := textOf(t, call(t, cs, "get_balance", map[string]any{"account": "cash"})); !strings.Contains(got, "50.00") {
		t.Errorf("get_balance = %q", got)
	}
}

// TestAddTransactionWrites verifies the tool actually persists to the Store.
func TestAddTransactionWrites(t *testing.T) {
	store := seed(t)
	cs := connect(t, store)
	res := call(t, cs, "add_transaction", map[string]any{
		"account": "cash", "amount": "-12.34", "date": "2026-07-15", "category": "food",
	})
	if res.IsError {
		t.Fatalf("add_transaction errored: %q", textOf(t, res))
	}
	txns, err := store.Transactions("cash", "2026-07")
	if err != nil {
		t.Fatal(err)
	}
	if len(txns) != 2 {
		t.Fatalf("want 2 txns, got %d", len(txns))
	}
	a, _, _ := store.Account("cash")
	if a.Balance != 3766 { // 5000 - 1234
		t.Errorf("balance = %d, want 3766", a.Balance)
	}
}

func TestErrors(t *testing.T) {
	cs := connect(t, seed(t))
	// Unknown account.
	if res := call(t, cs, "get_balance", map[string]any{"account": "nope"}); !res.IsError {
		t.Error("expected IsError for unknown account")
	}
	// Bad amount.
	if res := call(t, cs, "add_transaction", map[string]any{"account": "cash", "amount": "abc"}); !res.IsError {
		t.Error("expected IsError for bad amount")
	}
	// Bad month.
	if res := call(t, cs, "list_transactions", map[string]any{"account": "cash", "month": "2026-13-99"}); !res.IsError {
		t.Error("expected IsError for bad month")
	}
}

func TestSummary(t *testing.T) {
	cs := connect(t, seed(t))
	got := textOf(t, call(t, cs, "summary", map[string]any{"account": "cash", "month": "2026-07"}))
	if !strings.Contains(got, "salary") || !strings.Contains(got, "TOTAL") || !strings.Contains(got, "50.00") {
		t.Errorf("summary = %q", got)
	}
}
