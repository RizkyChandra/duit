// Package mcpserver exposes the duit ledger over an MCP stdio server.
package mcpserver

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Serve builds the MCP server, registers the ledger tools, and serves over
// stdio until ctx is done or stdin closes.
func Serve(ctx context.Context, store *ledger.Store) error {
	return newServer(store).Run(ctx, &mcp.StdioTransport{})
}

// newServer builds a server with all tools registered. Split out so tests can
// drive it over an in-memory transport.
func newServer(store *ledger.Store) *mcp.Server {
	s := mcp.NewServer(&mcp.Implementation{Name: "duit", Version: "v0.6.0"}, nil)
	h := handlers{store}
	mcp.AddTool(s, &mcp.Tool{Name: "list_accounts", Description: "List accounts with their balances."}, h.listAccounts)
	mcp.AddTool(s, &mcp.Tool{Name: "get_balance", Description: "Get an account's balance."}, h.getBalance)
	mcp.AddTool(s, &mcp.Tool{Name: "add_transaction", Description: "Add a transaction to an account."}, h.addTransaction)
	mcp.AddTool(s, &mcp.Tool{Name: "list_transactions", Description: "List an account's transactions for a month."}, h.listTransactions)
	mcp.AddTool(s, &mcp.Tool{Name: "summary", Description: "Per-category income/expense/net for a month."}, h.summary)
	mcp.AddTool(s, &mcp.Tool{Name: "budget_status", Description: "Per-category budget: spent vs limit for a month."}, h.budgetStatus)
	mcp.AddTool(s, &mcp.Tool{Name: "list_recurring", Description: "List recurring transaction rules."}, h.listRecurring)
	mcp.AddTool(s, &mcp.Tool{Name: "apply_recurring", Description: "Materialize recurring rules due up to a date (default today)."}, h.applyRecurring)
	mcp.AddTool(s, &mcp.Tool{Name: "net_worth", Description: "Total balance across accounts converted to one currency."}, h.netWorth)
	mcp.AddTool(s, &mcp.Tool{Name: "find_transactions", Description: "Search transactions by text, account, category, type, amount, or date."}, h.find)
	mcp.AddTool(s, &mcp.Tool{Name: "transfer", Description: "Move money between accounts (cross-currency auto-converts)."}, h.transfer)
	return s
}

type handlers struct{ store *ledger.Store }

func text(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: s}}}
}

// account looks an account up, returning a clear error if it's unknown.
func (h handlers) account(name string) (ledger.Account, error) {
	a, ok, err := h.store.Account(name)
	if err != nil {
		return a, err
	}
	if !ok {
		return a, fmt.Errorf("unknown account %q", name)
	}
	return a, nil
}

type noArgs struct{}

func (h handlers) listAccounts(_ context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	accts, err := h.store.LoadAccounts()
	if err != nil {
		return nil, nil, err
	}
	if len(accts) == 0 {
		return text("no accounts"), nil, nil
	}
	var b strings.Builder
	for _, a := range accts {
		fmt.Fprintf(&b, "%s\t%s %s\n", a.Name, a.Balance.Format(a.Decimals), a.Currency)
	}
	return text(strings.TrimRight(b.String(), "\n")), nil, nil
}

type accountArg struct {
	Account string `json:"account" jsonschema:"account name"`
}

func (h handlers) getBalance(_ context.Context, _ *mcp.CallToolRequest, in accountArg) (*mcp.CallToolResult, any, error) {
	a, err := h.account(in.Account)
	if err != nil {
		return nil, nil, err
	}
	return text(fmt.Sprintf("%s %s", a.Balance.Format(a.Decimals), a.Currency)), nil, nil
}

type addTxnArgs struct {
	Account  string `json:"account" jsonschema:"account name"`
	Amount   string `json:"amount" jsonschema:"decimal amount, positive income or negative expense, e.g. 12.34 or -5"`
	Date     string `json:"date,omitempty" jsonschema:"date YYYY-MM-DD, defaults to today"`
	Category string `json:"category,omitempty" jsonschema:"category"`
	Note     string `json:"note,omitempty" jsonschema:"free-form note"`
}

func (h handlers) addTransaction(_ context.Context, _ *mcp.CallToolRequest, in addTxnArgs) (*mcp.CallToolResult, any, error) {
	a, err := h.account(in.Account)
	if err != nil {
		return nil, nil, err
	}
	amount, err := ledger.ParseMoney(in.Amount, a.Decimals)
	if err != nil {
		return nil, nil, fmt.Errorf("bad amount %q: %w", in.Amount, err)
	}
	date := in.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	t, err := h.store.AddTransaction(a.Name, ledger.Transaction{
		Date: date, Amount: amount, Category: in.Category, Note: in.Note,
	})
	if err != nil {
		return nil, nil, err
	}
	// Re-read for the fresh balance.
	a, _ = h.account(a.Name)
	return text(fmt.Sprintf("added %s; balance %s %s", t.ID, a.Balance.Format(a.Decimals), a.Currency)), nil, nil
}

type monthArgs struct {
	Account string `json:"account" jsonschema:"account name"`
	Month   string `json:"month,omitempty" jsonschema:"month YYYY-MM, defaults to current month"`
}

func (h handlers) listTransactions(_ context.Context, _ *mcp.CallToolRequest, in monthArgs) (*mcp.CallToolResult, any, error) {
	a, err := h.account(in.Account)
	if err != nil {
		return nil, nil, err
	}
	month, err := defaultMonth(in.Month)
	if err != nil {
		return nil, nil, err
	}
	txns, err := h.store.Transactions(a.Name, month)
	if err != nil {
		return nil, nil, err
	}
	if len(txns) == 0 {
		return text(fmt.Sprintf("no transactions for %s in %s", a.Name, month)), nil, nil
	}
	var b strings.Builder
	for _, t := range txns {
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\n", t.Date, t.ID, t.Amount.Format(a.Decimals), t.Category, t.Note)
	}
	return text(strings.TrimRight(b.String(), "\n")), nil, nil
}

func (h handlers) summary(_ context.Context, _ *mcp.CallToolRequest, in monthArgs) (*mcp.CallToolResult, any, error) {
	month, err := defaultMonth(in.Month)
	if err != nil {
		return nil, nil, err
	}
	var accts []ledger.Account
	if in.Account != "" {
		a, err := h.account(in.Account)
		if err != nil {
			return nil, nil, err
		}
		accts = []ledger.Account{a}
	} else {
		if accts, err = h.store.LoadAccounts(); err != nil {
			return nil, nil, err
		}
	}
	var b strings.Builder
	for _, a := range accts {
		txns, err := h.store.Transactions(a.Name, month)
		if err != nil {
			return nil, nil, err
		}
		fmt.Fprintf(&b, "%s (%s) %s\n", a.Name, a.Currency, month)
		writeSummary(&b, txns, a.Decimals)
	}
	if b.Len() == 0 {
		return text("no accounts"), nil, nil
	}
	return text(strings.TrimRight(b.String(), "\n")), nil, nil
}

// writeSummary appends per-category income/expense/net lines plus a total.
func writeSummary(b *strings.Builder, txns []ledger.Transaction, decimals int) {
	type totals struct{ income, expense ledger.Money }
	byCat := map[string]*totals{}
	var order []string
	for _, t := range txns {
		if t.Transfer != "" {
			continue // transfers are not income/expense
		}
		for _, ln := range t.Lines() { // attribute split parts to their own categories
			cat := ln.Category
			if cat == "" {
				cat = "(uncategorized)"
			}
			tot, ok := byCat[cat]
			if !ok {
				tot = &totals{}
				byCat[cat] = tot
				order = append(order, cat)
			}
			if ln.Amount >= 0 {
				tot.income += ln.Amount
			} else {
				tot.expense += ln.Amount // negative
			}
		}
	}
	sort.Strings(order)
	var income, expense ledger.Money
	for _, cat := range order {
		t := byCat[cat]
		income += t.income
		expense += t.expense
		fmt.Fprintf(b, "  %s\tincome %s\texpense %s\tnet %s\n",
			cat, t.income.Format(decimals), (-t.expense).Format(decimals), (t.income + t.expense).Format(decimals))
	}
	fmt.Fprintf(b, "  TOTAL\tincome %s\texpense %s\tnet %s\n",
		income.Format(decimals), (-expense).Format(decimals), (income + expense).Format(decimals))
}

// defaultMonth returns month or the current YYYY-MM if empty, validating format.
func defaultMonth(month string) (string, error) {
	if month == "" {
		return time.Now().Format("2006-01"), nil
	}
	if _, err := time.Parse("2006-01", month); err != nil {
		return "", fmt.Errorf("bad month %q (want YYYY-MM)", month)
	}
	return month, nil
}
