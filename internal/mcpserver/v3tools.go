package mcpserver

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/RizkyChandra/duit/internal/ledger"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type findArgs struct {
	Text     string `json:"text,omitempty" jsonschema:"substring of note/category"`
	Account  string `json:"account,omitempty" jsonschema:"limit to one account"`
	Category string `json:"category,omitempty" jsonschema:"exact category (incl. splits)"`
	Type     string `json:"type,omitempty" jsonschema:"income|expense|transfer"`
	Min      string `json:"min,omitempty" jsonschema:"minimum amount magnitude, decimal"`
	Max      string `json:"max,omitempty" jsonschema:"maximum amount magnitude, decimal"`
	From     string `json:"from,omitempty" jsonschema:"YYYY-MM-DD inclusive"`
	To       string `json:"to,omitempty" jsonschema:"YYYY-MM-DD inclusive"`
	Month    string `json:"month,omitempty" jsonschema:"YYYY-MM"`
}

func (h handlers) find(_ context.Context, _ *mcp.CallToolRequest, in findArgs) (*mcp.CallToolResult, any, error) {
	dec := budgetDecimals(h.store)
	f := ledger.FindFilter{Text: in.Text, Account: in.Account, Category: in.Category, Type: in.Type, From: in.From, To: in.To}
	if in.Month != "" {
		f.From, f.To = in.Month+"-01", in.Month+"-31"
	}
	if in.Min != "" {
		m, err := ledger.ParseMoney(in.Min, dec)
		if err != nil {
			return nil, nil, fmt.Errorf("bad min: %w", err)
		}
		f.Min = &m
	}
	if in.Max != "" {
		m, err := ledger.ParseMoney(in.Max, dec)
		if err != nil {
			return nil, nil, fmt.Errorf("bad max: %w", err)
		}
		f.Max = &m
	}
	found, err := h.store.Find(f)
	if err != nil {
		return nil, nil, err
	}
	if len(found) == 0 {
		return text("no matching transactions"), nil, nil
	}
	accts, _ := h.store.LoadAccounts()
	decOf := map[string]int{}
	for _, a := range accts {
		decOf[a.Name] = a.Decimals
	}
	var b strings.Builder
	for _, x := range found {
		cat := x.Category
		if len(x.Splits) > 0 {
			parts := make([]string, len(x.Splits))
			for i, s := range x.Splits {
				parts[i] = s.Category
			}
			cat = "split:" + strings.Join(parts, ",")
		}
		fmt.Fprintf(&b, "%s\t%s\t%s\t%s\t%s\t%s\n",
			x.Date, x.Account, x.Amount.Format(decOf[x.Account]), cat, x.Note, x.ID)
	}
	return text(strings.TrimRight(b.String(), "\n")), nil, nil
}

type transferArgs struct {
	From       string `json:"from" jsonschema:"source account"`
	To         string `json:"to" jsonschema:"destination account"`
	Amount     string `json:"amount" jsonschema:"decimal amount in the source account's currency"`
	DestAmount string `json:"dest_amount,omitempty" jsonschema:"exact amount received (overrides fx conversion)"`
	Date       string `json:"date,omitempty" jsonschema:"YYYY-MM-DD, default today"`
	Note       string `json:"note,omitempty"`
}

func (h handlers) transfer(_ context.Context, _ *mcp.CallToolRequest, in transferArgs) (*mcp.CallToolResult, any, error) {
	from, err := h.account(in.From)
	if err != nil {
		return nil, nil, err
	}
	to, err := h.account(in.To)
	if err != nil {
		return nil, nil, err
	}
	amt, err := ledger.ParseMoney(in.Amount, from.Decimals)
	if err != nil {
		return nil, nil, fmt.Errorf("bad amount: %w", err)
	}
	var destPtr *ledger.Money
	if in.DestAmount != "" {
		d, err := ledger.ParseMoney(in.DestAmount, to.Decimals)
		if err != nil {
			return nil, nil, fmt.Errorf("bad dest_amount: %w", err)
		}
		destPtr = &d
	}
	date := in.Date
	if date == "" {
		date = time.Now().Format("2006-01-02")
	}
	src, dst, err := h.store.Transfer(from.Name, to.Name, amt, destPtr, date, in.Note)
	if err != nil {
		return nil, nil, err
	}
	return text(fmt.Sprintf("transferred %s %s from %s → %s %s to %s",
		src.Format(from.Decimals), from.Currency, from.Name,
		dst.Format(to.Decimals), to.Currency, to.Name)), nil, nil
}
