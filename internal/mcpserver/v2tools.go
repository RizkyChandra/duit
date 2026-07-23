package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type monthOnlyArgs struct {
	Month string `json:"month,omitempty" jsonschema:"month YYYY-MM, defaults to current month"`
}

func (h handlers) budgetStatus(_ context.Context, _ *mcp.CallToolRequest, in monthOnlyArgs) (*mcp.CallToolResult, any, error) {
	month, err := defaultMonth(in.Month)
	if err != nil {
		return nil, nil, err
	}
	lines, err := h.store.BudgetStatus(month)
	if err != nil {
		return nil, nil, err
	}
	if len(lines) == 0 {
		return text("no budgets set"), nil, nil
	}
	dec := budgetDecimals(h.store)
	var b strings.Builder
	for _, l := range lines {
		flag := ""
		if l.Over {
			flag = " OVER"
		}
		fmt.Fprintf(&b, "%s\tlimit %s\tspent %s\tremaining %s%s\n",
			l.Category, l.Limit.Format(dec), l.Spent.Format(dec), l.Remaining.Format(dec), flag)
	}
	return text(strings.TrimRight(b.String(), "\n")), nil, nil
}

func (h handlers) listRecurring(_ context.Context, _ *mcp.CallToolRequest, _ noArgs) (*mcp.CallToolResult, any, error) {
	rules, err := h.store.LoadRecurring()
	if err != nil {
		return nil, nil, err
	}
	if len(rules) == 0 {
		return text("no recurring rules"), nil, nil
	}
	var b strings.Builder
	for _, r := range rules {
		dec := 2
		if a, ok, _ := h.store.Account(r.Account); ok {
			dec = a.Decimals
		}
		last := r.LastApplied
		if last == "" {
			last = "never"
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s every %d\tday %d\tstart %s\tlast %s\n",
			r.ID, r.Account, r.Amount.Format(dec), r.Cadence, r.Interval, r.Day, r.Start, last)
	}
	return text(strings.TrimRight(b.String(), "\n")), nil, nil
}

type applyArgs struct {
	Until string `json:"until,omitempty" jsonschema:"apply through this date YYYY-MM-DD, defaults to today"`
}

func (h handlers) applyRecurring(_ context.Context, _ *mcp.CallToolRequest, in applyArgs) (*mcp.CallToolResult, any, error) {
	until := in.Until
	if until == "" {
		until = time.Now().Format("2006-01-02")
	}
	n, err := h.store.ApplyRecurring(until)
	if err != nil {
		return nil, nil, err
	}
	return text(fmt.Sprintf("applied %d recurring transaction(s) up to %s", n, until)), nil, nil
}

type netWorthArgs struct {
	In string `json:"in,omitempty" jsonschema:"target currency, defaults to the rate table's base"`
}

func (h handlers) netWorth(_ context.Context, _ *mcp.CallToolRequest, in netWorthArgs) (*mcp.CallToolResult, any, error) {
	rates, err := h.store.LoadRates()
	if err != nil {
		return nil, nil, err
	}
	target := strings.ToUpper(in.In)
	if target == "" {
		target = rates.Base
	}
	if target == "" {
		return nil, nil, fmt.Errorf("no target currency; set fx rates or pass `in`")
	}
	accts, err := h.store.LoadAccounts()
	if err != nil {
		return nil, nil, err
	}
	dec := ledger.CurrencyDecimals(target)
	var total ledger.Money
	var missing []string
	var b strings.Builder
	for _, a := range accts {
		conv, err := rates.Convert(a.Balance, a.Currency, target)
		if err != nil {
			missing = append(missing, a.Currency)
			continue
		}
		total += conv
		fmt.Fprintf(&b, "%s\t%s %s = %s %s\n", a.Name, a.Balance.Format(a.Decimals), a.Currency, conv.Format(dec), target)
	}
	fmt.Fprintf(&b, "TOTAL\t%s %s", total.Format(dec), target)
	if len(missing) > 0 {
		fmt.Fprintf(&b, "\n(no rate for: %s)", strings.Join(missing, ", "))
	}
	return text(b.String()), nil, nil
}

// budgetDecimals infers decimal precision for budget amounts from the first
// account (budgets are single-currency until v0.3 FX), defaulting to 2.
func budgetDecimals(store *ledger.Store) int {
	if accts, err := store.LoadAccounts(); err == nil && len(accts) > 0 {
		return accts[0].Decimals
	}
	return 2
}
