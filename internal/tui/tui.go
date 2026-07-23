// Package tui is an interactive terminal UI over the ledger core (roadmap R6).
package tui

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/RizkyChandra/duit/internal/ledger"

	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// field is a minimal single-line text input. bubbles/textinput would do this,
// but its clipboard transitive dep is absent from this module's go.sum and the
// isolation rules forbid touching go.sum — so a ~20-line field it is.
// ponytail: no scrolling/selection; swap in bubbles/textinput if go.sum gains atotto/clipboard.
type field struct {
	label   string
	value   string
	focused bool
}

func (f *field) update(km tea.KeyMsg) {
	switch km.Type {
	case tea.KeyRunes, tea.KeySpace:
		f.value += string(km.Runes)
	case tea.KeyBackspace:
		if n := len(f.value); n > 0 {
			// Trim one rune, not one byte.
			r := []rune(f.value)
			f.value = string(r[:len(r)-1])
		}
	}
}

func (f field) view() string {
	cur := ""
	if f.focused {
		cur = cursorStyle.Render("_")
	}
	lbl := fmt.Sprintf("%-20s ", f.label)
	if f.focused {
		lbl = cursorStyle.Render(lbl)
	}
	return lbl + f.value + cur
}

type screen int

const (
	screenAccounts screen = iota
	screenTxns
	screenForm
	screenBudget
	screenFX
	screenTransfer
)

const dateFmt = "2006-01-02"

var (
	headerStyle = lipgloss.NewStyle().Bold(true).Padding(0, 1).
			Foreground(lipgloss.Color("15")).Background(lipgloss.Color("63"))
	cursorStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("212"))
	statusStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
)

// Model is the root bubbletea model. Exported so it can be driven in tests.
type Model struct {
	store *ledger.Store
	sync  func() error

	screen screen
	status string
	err    error

	accounts []ledger.Account
	accCur   int

	acct  ledger.Account // selected account
	month string         // month being viewed (YYYY-MM)
	txns  []ledger.Transaction
	table table.Model

	// add/edit form
	inputs    []field
	formFocus int
	editing   bool
	editID    string
	editMonth string

	// budget/fx screens
	budgetMonth string
	budgets     []ledger.BudgetLine
	rates       ledger.Rates

	// inline prompt overlay (budget/fx edits); nil when not prompting
	prompt      []field
	promptKind  string
	promptFocus int

	// transfer form
	transDest int // index into m.accounts of the destination
}

// New builds the root model, loading the account list. Errors are surfaced in
// the status line rather than returned so the program can still start.
func New(store *ledger.Store, sync func() error) Model {
	m := Model{store: store, sync: sync, screen: screenAccounts}
	m.reloadAccounts()
	return m
}

// Run launches the bubbletea program and blocks until quit. sync is the git
// callback wired by the CLI; a nil sync makes the sync key a no-op.
func Run(store *ledger.Store, sync func() error) error {
	_, err := tea.NewProgram(New(store, sync), tea.WithAltScreen()).Run()
	return err
}

func (m Model) Init() tea.Cmd { return nil }

func (m *Model) reloadAccounts() {
	accts, err := m.store.LoadAccounts()
	if err != nil {
		m.err = err
		return
	}
	m.accounts = accts
	if m.accCur >= len(accts) {
		m.accCur = 0
	}
}

// latestMonth returns the newest month with data, else the current calendar month.
func (m *Model) latestMonth(account string) string {
	months, _ := m.store.Months(account)
	if len(months) > 0 {
		return months[len(months)-1]
	}
	return time.Now().Format("2006-01")
}

func (m *Model) openAccount(a ledger.Account) {
	m.acct = a
	m.month = m.latestMonth(a.Name)
	m.reloadTxns()
	m.screen = screenTxns
	m.status = ""
	m.err = nil
}

func (m *Model) reloadTxns() {
	// Refresh the account (its cached balance changes after mutations).
	if a, ok, err := m.store.Account(m.acct.Name); err == nil && ok {
		m.acct = a
	}
	txns, err := m.store.Transactions(m.acct.Name, m.month)
	if err != nil {
		m.err = err
	}
	m.txns = txns
	rows := make([]table.Row, len(txns))
	for i, t := range txns {
		rows[i] = table.Row{t.Date, t.Amount.Format(m.acct.Decimals), t.Category, t.Note}
	}
	cols := []table.Column{
		{Title: "Date", Width: 12},
		{Title: "Amount", Width: 14},
		{Title: "Category", Width: 16},
		{Title: "Note", Width: 28},
	}
	cur := m.table.Cursor()
	m.table = table.New(
		table.WithColumns(cols),
		table.WithRows(rows),
		table.WithFocused(true),
		table.WithHeight(12),
	)
	if cur >= 0 && cur < len(rows) {
		m.table.SetCursor(cur)
	}
}

func (m *Model) newForm(t ledger.Transaction) {
	labels := []string{"Amount (e.g. -12.50)", "Category", "Note", "Date (YYYY-MM-DD)"}
	vals := []string{"", "", "", time.Now().Format(dateFmt)}
	if m.editing {
		vals = []string{t.Amount.Format(m.acct.Decimals), t.Category, t.Note, t.Date}
	}
	m.inputs = make([]field, 4)
	for i := range m.inputs {
		m.inputs[i] = field{label: labels[i], value: vals[i]}
	}
	m.formFocus = 0
	m.inputs[0].focused = true
}

func (m *Model) submitForm() error {
	amt, err := ledger.ParseMoney(m.inputs[0].value, m.acct.Decimals)
	if err != nil {
		return err
	}
	nt := ledger.Transaction{
		Date:     m.inputs[3].value,
		Amount:   amt,
		Category: m.inputs[1].value,
		Note:     m.inputs[2].value,
	}
	if m.editing {
		// edit = remove + add; keep the id so history is stable.
		if err := m.store.RemoveTransaction(m.acct.Name, m.editMonth, m.editID); err != nil {
			return err
		}
		nt.ID = m.editID
	}
	if _, err := m.store.AddTransaction(m.acct.Name, nt); err != nil {
		return err
	}
	// If the date moved to another month, follow it so the entry stays visible.
	if len(nt.Date) >= 7 {
		m.month = nt.Date[:7]
	}
	return nil
}

// Update drives the state machine. It is the standard tea.Model entry point.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		// Non-key messages (e.g. table tick) go to the table only.
		if m.screen == screenTxns {
			var cmd tea.Cmd
			m.table, cmd = m.table.Update(msg)
			return m, cmd
		}
		return m, nil
	}
	if km.String() == "ctrl+c" {
		return m, tea.Quit
	}
	switch m.screen {
	case screenAccounts:
		return m.updateAccounts(km)
	case screenTxns:
		return m.updateTxns(km)
	case screenForm:
		return m.updateForm(km)
	case screenBudget, screenFX:
		return m.updateBudgetFX(km)
	case screenTransfer:
		return m.updateTransfer(km)
	}
	return m, nil
}

func (m Model) updateAccounts(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "q":
		return m, tea.Quit
	case "up", "k":
		if m.accCur > 0 {
			m.accCur--
		}
	case "down", "j":
		if m.accCur < len(m.accounts)-1 {
			m.accCur++
		}
	case "enter":
		if len(m.accounts) > 0 {
			m.openAccount(m.accounts[m.accCur])
		}
	case "b":
		m.budgetMonth = time.Now().Format("2006-01")
		m.budgets, m.err = m.store.BudgetStatus(m.budgetMonth)
		m.screen = screenBudget
	case "f":
		m.rates, m.err = m.store.LoadRates()
		m.screen = screenFX
	case "t":
		if len(m.accounts) < 2 {
			m.status = "need at least 2 accounts to transfer"
		} else {
			m.transDest = m.accCur
			m.cycleDest(1) // first account that isn't the source
			m.inputs = []field{{label: "Amount", focused: true}, {label: "Note"}}
			m.formFocus = 0
			m.status, m.err = "", nil
			m.screen = screenTransfer
		}
	}
	return m, nil
}

// cycleDest advances the destination index to the next account that isn't the
// source (m.accCur), wrapping around.
func (m *Model) cycleDest(dir int) {
	n := len(m.accounts)
	for k := 0; k < n; k++ {
		m.transDest = (m.transDest + dir + n) % n
		if m.transDest != m.accCur {
			return
		}
	}
}

func (m Model) updateBudgetFX(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.prompt != nil {
		return m.updatePrompt(km)
	}
	switch km.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		m.screen = screenAccounts
	case "a", "s":
		if m.screen == screenBudget {
			m.startPrompt("budgetset", "Category", "Limit")
		} else {
			m.startPrompt("fxset", "Currency CODE", "Rate")
		}
	case "d":
		if m.screen == screenBudget {
			m.startPrompt("budgetdel", "Category")
		} else {
			m.startPrompt("fxdel", "Currency CODE")
		}
	}
	return m, nil
}

func (m *Model) startPrompt(kind string, labels ...string) {
	m.prompt = make([]field, len(labels))
	for i, l := range labels {
		m.prompt[i] = field{label: l}
	}
	m.prompt[0].focused = true
	m.promptFocus = 0
	m.promptKind = kind
	m.err = nil
}

func (m Model) updatePrompt(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "esc":
		m.prompt = nil
	case "enter":
		if err := m.submitPrompt(); err != nil {
			m.err = err
		} else {
			m.err = nil
			m.prompt = nil
		}
	case "tab", "down":
		m.promptFocus = (m.promptFocus + 1) % len(m.prompt)
		m.focusPrompt()
	case "shift+tab", "up":
		m.promptFocus = (m.promptFocus - 1 + len(m.prompt)) % len(m.prompt)
		m.focusPrompt()
	default:
		m.prompt[m.promptFocus].update(km)
	}
	return m, nil
}

func (m *Model) focusPrompt() {
	for i := range m.prompt {
		m.prompt[i].focused = i == m.promptFocus
	}
}

func (m *Model) submitPrompt() error {
	switch m.promptKind {
	case "budgetset":
		lim, err := ledger.ParseMoney(m.prompt[1].value, ledger.CurrencyDecimals(m.rates.Base))
		if err != nil {
			return err
		}
		if err := m.store.SetBudget(strings.TrimSpace(m.prompt[0].value), lim); err != nil {
			return err
		}
		m.status = "budget set"
	case "budgetdel":
		if err := m.store.RemoveBudget(strings.TrimSpace(m.prompt[0].value)); err != nil {
			return err
		}
		m.status = "budget removed"
	case "fxset":
		rate, err := strconv.ParseFloat(strings.TrimSpace(m.prompt[1].value), 64)
		if err != nil {
			return err
		}
		r, err := m.store.LoadRates()
		if err != nil {
			return err
		}
		if r.Rates == nil {
			r.Rates = map[string]float64{}
		}
		r.Rates[strings.ToUpper(strings.TrimSpace(m.prompt[0].value))] = rate
		if r.Base == "" && len(m.accounts) > 0 {
			r.Base = m.accounts[0].Currency
			r.Rates[r.Base] = 1
		}
		if err := m.store.SaveRates(r); err != nil {
			return err
		}
		m.status = "rate set"
	case "fxdel":
		r, err := m.store.LoadRates()
		if err != nil {
			return err
		}
		code := strings.ToUpper(strings.TrimSpace(m.prompt[0].value))
		if code == r.Base {
			return fmt.Errorf("cannot remove base currency %s", code)
		}
		delete(r.Rates, code)
		if err := m.store.SaveRates(r); err != nil {
			return err
		}
		m.status = "rate removed"
	}
	// Reload whichever screen is underneath.
	if m.screen == screenBudget {
		m.budgets, _ = m.store.BudgetStatus(m.budgetMonth)
	} else {
		m.rates, _ = m.store.LoadRates()
	}
	return nil
}

func (m Model) updateTransfer(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "esc":
		m.screen = screenAccounts
	case "left":
		m.cycleDest(-1)
	case "right":
		m.cycleDest(1)
	case "tab", "down":
		m.focusInput(m.formFocus + 1)
	case "shift+tab", "up":
		m.focusInput(m.formFocus - 1)
	case "enter":
		if err := m.submitTransfer(); err != nil {
			m.err = err
		} else {
			m.err = nil
			m.reloadAccounts()
			m.screen = screenAccounts
		}
	default:
		m.inputs[m.formFocus].update(km)
	}
	return m, nil
}

func (m *Model) submitTransfer() error {
	src, dst := m.accounts[m.accCur], m.accounts[m.transDest]
	amt, err := ledger.ParseMoney(m.inputs[0].value, src.Decimals)
	if err != nil {
		return err
	}
	got, sent, err := m.store.Transfer(src.Name, dst.Name, amt, nil, time.Now().Format(dateFmt), m.inputs[1].value)
	if err != nil {
		return err
	}
	m.status = fmt.Sprintf("transferred %s from %s -> %s to %s",
		got.Format(src.Decimals), src.Name, sent.Format(dst.Decimals), dst.Name)
	return nil
}

func (m Model) updateTxns(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "q":
		return m, tea.Quit
	case "esc":
		m.reloadAccounts()
		m.screen = screenAccounts
		return m, nil
	case "a":
		m.editing = false
		m.newForm(ledger.Transaction{})
		m.screen = screenForm
		return m, nil
	case "e":
		if i := m.table.Cursor(); i >= 0 && i < len(m.txns) {
			m.editing = true
			m.editID = m.txns[i].ID
			m.editMonth = m.month
			m.newForm(m.txns[i])
			m.screen = screenForm
			return m, nil
		}
	case "d":
		if i := m.table.Cursor(); i >= 0 && i < len(m.txns) {
			if err := m.store.RemoveTransaction(m.acct.Name, m.month, m.txns[i].ID); err != nil {
				m.err = err
			} else {
				m.status = "deleted"
				m.reloadTxns()
			}
		}
		return m, nil
	case "s":
		if m.sync != nil {
			if err := m.sync(); err != nil {
				m.err = fmt.Errorf("sync: %w", err)
			} else {
				m.status = "synced"
				m.reloadTxns()
			}
		} else {
			m.status = "sync not configured"
		}
		return m, nil
	}
	var cmd tea.Cmd
	m.table, cmd = m.table.Update(km)
	return m, cmd
}

func (m Model) updateForm(km tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch km.String() {
	case "esc":
		m.screen = screenTxns
		return m, nil
	case "enter":
		if err := m.submitForm(); err != nil {
			m.err = err
			return m, nil
		}
		m.err = nil
		if m.editing {
			m.status = "updated"
		} else {
			m.status = "added"
		}
		m.reloadTxns()
		m.screen = screenTxns
		return m, nil
	case "tab", "down":
		m.focusInput(m.formFocus + 1)
		return m, nil
	case "shift+tab", "up":
		m.focusInput(m.formFocus - 1)
		return m, nil
	}
	m.inputs[m.formFocus].update(km)
	return m, nil
}

func (m *Model) focusInput(i int) {
	if i < 0 {
		i = len(m.inputs) - 1
	}
	i %= len(m.inputs)
	for j := range m.inputs {
		m.inputs[j].focused = j == i
	}
	m.formFocus = i
}

// View renders the current screen.
func (m Model) View() string {
	switch m.screen {
	case screenTxns:
		return m.viewTxns()
	case screenForm:
		return m.viewForm()
	case screenBudget:
		return m.viewBudget()
	case screenFX:
		return m.viewFX()
	case screenTransfer:
		return m.viewTransfer()
	default:
		return m.viewAccounts()
	}
}

// promptView renders the inline edit overlay for the budget/fx screens.
func (m Model) promptView() string {
	if m.prompt == nil {
		return ""
	}
	s := "\n"
	for i := range m.prompt {
		s += m.prompt[i].view() + "\n"
	}
	return s + statusStyle.Render("enter confirm · esc cancel")
}

func (m Model) viewTransfer() string {
	src, dst := m.accounts[m.accCur], m.accounts[m.transDest]
	s := headerStyle.Render("Transfer from "+src.Name) + "\n\n"
	s += fmt.Sprintf("  To: %s (%s)   ", dst.Name, dst.Currency) + cursorStyle.Render("←/→ change") + "\n\n"
	for i := range m.inputs {
		s += m.inputs[i].view() + "\n"
	}
	s += "\n" + statusStyle.Render("←/→ dest · tab move · enter send · esc cancel")
	return s + m.footer()
}

func (m Model) viewAccounts() string {
	s := headerStyle.Render("duit — accounts") + "\n\n"
	if len(m.accounts) == 0 {
		s += "  (no accounts)\n"
	}
	for i, a := range m.accounts {
		cursor := "  "
		line := fmt.Sprintf("%-16s %14s", a.Name, a.Balance.Format(a.Decimals))
		if i == m.accCur {
			cursor = cursorStyle.Render("> ")
			line = cursorStyle.Render(line)
		}
		s += cursor + line + "\n"
	}
	s += "\n" + statusStyle.Render("up/down·k/j move · enter open · t transfer · b budget · f fx · q quit")
	return s + m.footer()
}

func (m Model) viewBudget() string {
	dec := ledger.CurrencyDecimals(m.rates.Base) // rates.Base may be empty -> 2
	s := headerStyle.Render("Budgets — "+m.budgetMonth) + "\n\n"
	if len(m.budgets) == 0 {
		s += "  (no budgets set)\n"
	} else {
		s += fmt.Sprintf("  %-16s %12s %12s %12s\n", "CATEGORY", "LIMIT", "SPENT", "REMAINING")
		for _, b := range m.budgets {
			line := fmt.Sprintf("  %-16s %12s %12s %12s", b.Category,
				b.Limit.Format(dec), b.Spent.Format(dec), b.Remaining.Format(dec))
			if b.Over {
				line = errStyle.Render(line + "  OVER")
			}
			s += line + "\n"
		}
	}
	s += "\n" + statusStyle.Render("s set · d remove · esc back · q quit")
	s += m.promptView()
	return s + m.footer()
}

func (m Model) viewFX() string {
	s := headerStyle.Render("FX rates — base "+m.rates.Base) + "\n\n"
	if len(m.rates.Rates) == 0 {
		s += "  (no rates set)\n"
	} else {
		codes := make([]string, 0, len(m.rates.Rates))
		for c := range m.rates.Rates {
			codes = append(codes, c)
		}
		sort.Strings(codes)
		for _, c := range codes {
			s += fmt.Sprintf("  1 %s = %g %s\n", m.rates.Base, m.rates.Rates[c], c)
		}
	}
	s += "\n" + statusStyle.Render("s set · d remove · esc back · q quit")
	s += m.promptView()
	return s + m.footer()
}

func (m Model) viewTxns() string {
	head := fmt.Sprintf("%s  [%s]  balance %s", m.acct.Name, m.month, m.acct.Balance.Format(m.acct.Decimals))
	s := headerStyle.Render(head) + "\n\n"
	s += m.table.View() + "\n\n"
	s += statusStyle.Render("k/j move · a add · e edit · d delete · s sync · esc back · q quit")
	return s + m.footer()
}

func (m Model) viewForm() string {
	title := "Add transaction"
	if m.editing {
		title = "Edit transaction"
	}
	s := headerStyle.Render(m.acct.Name+" — "+title) + "\n\n"
	for i := range m.inputs {
		s += m.inputs[i].view() + "\n"
	}
	s += "\n" + statusStyle.Render("tab/up/down move · enter save · esc cancel")
	return s + m.footer()
}

func (m Model) footer() string {
	if m.err != nil {
		return "\n" + errStyle.Render("error: "+m.err.Error())
	}
	if m.status != "" {
		return "\n" + statusStyle.Render(m.status)
	}
	return ""
}
