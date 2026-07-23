package ledger

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Store reads and writes the ledger's JSON files under Dir.
type Store struct{ Dir string }

// MonthFile holds one account-month of transactions plus the running balance at
// the start (Opening) and end (Closing) of that month.
type MonthFile struct {
	Opening      Money         `json:"opening"`
	Closing      Money         `json:"closing"`
	Transactions []Transaction `json:"transactions"`
}

func (s *Store) accountsPath() string { return filepath.Join(s.Dir, "accounts.json") }
func (s *Store) monthPath(account, month string) string {
	return filepath.Join(s.Dir, "txns", account, month+".json")
}

// --- accounts ---

// LoadAccounts returns the account registry, or nil if none exists yet.
func (s *Store) LoadAccounts() ([]Account, error) {
	var accts []Account
	err := readJSON(s.accountsPath(), &accts)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return accts, err
	}
	// Fail closed if the registry (possibly pulled from a remote) contains a name
	// that would escape the data dir when turned into a path.
	for _, a := range accts {
		if verr := validAccountName(a.Name); verr != nil {
			return nil, fmt.Errorf("refusing to load accounts.json: %w", verr)
		}
	}
	return accts, nil
}

func (s *Store) SaveAccounts(accts []Account) error {
	return writeJSON(s.accountsPath(), accts)
}

// validAccountName rejects names that aren't a single clean path element, so an
// account name can never escape the data dir when used to build a file path
// (defends against both a typo'd `../` name and a hostile remote's accounts.json).
func validAccountName(name string) error {
	if name == "" || name == "." || name == ".." || name != filepath.Base(name) || strings.ContainsAny(name, `/\`) {
		return fmt.Errorf("invalid account name %q (no path separators or ..)", name)
	}
	return nil
}

// AddAccount appends a, erroring if the name is already taken.
func (s *Store) AddAccount(a Account) error {
	if err := validAccountName(a.Name); err != nil {
		return err
	}
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	accts, err := s.LoadAccounts()
	if err != nil {
		return err
	}
	for _, x := range accts {
		if x.Name == a.Name {
			return fmt.Errorf("account %q already exists", a.Name)
		}
	}
	return s.SaveAccounts(append(accts, a))
}

// Account looks up an account by name.
func (s *Store) Account(name string) (Account, bool, error) {
	accts, err := s.LoadAccounts()
	if err != nil {
		return Account{}, false, err
	}
	for _, a := range accts {
		if a.Name == name {
			return a, true, nil
		}
	}
	return Account{}, false, nil
}

// --- transactions ---

// LoadMonth returns an account's month file, or an empty one if absent.
func (s *Store) LoadMonth(account, month string) (MonthFile, error) {
	var mf MonthFile
	err := readJSON(s.monthPath(account, month), &mf)
	if errors.Is(err, os.ErrNotExist) {
		return MonthFile{}, nil
	}
	return mf, err
}

// Transactions returns an account's transactions for a month (YYYY-MM).
func (s *Store) Transactions(account, month string) ([]Transaction, error) {
	mf, err := s.LoadMonth(account, month)
	return mf.Transactions, err
}

// Months lists the months (YYYY-MM, chronological) that have a file for account.
func (s *Store) Months(account string) ([]string, error) {
	dir := filepath.Join(s.Dir, "txns", account)
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var months []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			months = append(months, strings.TrimSuffix(e.Name(), ".json"))
		}
	}
	sort.Strings(months) // YYYY-MM sorts chronologically
	return months, nil
}

// AddTransaction appends t to its month file (derived from t.Date), then
// recomputes opening/closing for all months and the cached balance.
func (s *Store) AddTransaction(account string, t Transaction) (Transaction, error) {
	if !validDate(t.Date) {
		return Transaction{}, fmt.Errorf("invalid date %q (want YYYY-MM-DD)", t.Date)
	}
	if err := validateSplits(t); err != nil {
		return Transaction{}, err
	}
	unlock, err := s.lock()
	if err != nil {
		return Transaction{}, err
	}
	defer unlock()
	if t.ID == "" {
		t.ID = newID()
	}
	month := t.Date[:7]
	mf, err := s.LoadMonth(account, month)
	if err != nil {
		return Transaction{}, err
	}
	mf.Transactions = append(mf.Transactions, t)
	if err := writeJSON(s.monthPath(account, month), mf); err != nil {
		return Transaction{}, err
	}
	if _, err := s.recompute(account); err != nil {
		return Transaction{}, err
	}
	return t, nil
}

// RemoveAccount deletes an account from the registry along with all of its
// transaction files.
func (s *Store) RemoveAccount(name string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	accts, err := s.LoadAccounts()
	if err != nil {
		return err
	}
	out := accts[:0:0]
	found := false
	for _, a := range accts {
		if a.Name == name {
			found = true
			continue
		}
		out = append(out, a)
	}
	if !found {
		return fmt.Errorf("account %q not found", name)
	}
	if err := s.SaveAccounts(out); err != nil {
		return err
	}
	return os.RemoveAll(filepath.Join(s.Dir, "txns", name))
}

// RemoveTransaction deletes the transaction with id from an account's month
// file, then recomputes balances. Editing = RemoveTransaction + AddTransaction.
func (s *Store) RemoveTransaction(account, month, id string) error {
	unlock, err := s.lock()
	if err != nil {
		return err
	}
	defer unlock()
	mf, err := s.LoadMonth(account, month)
	if err != nil {
		return err
	}
	out := mf.Transactions[:0:0]
	found := false
	for _, t := range mf.Transactions {
		if t.ID == id {
			found = true
			continue
		}
		out = append(out, t)
	}
	if !found {
		return fmt.Errorf("transaction %q not found in %s", id, month)
	}
	mf.Transactions = out
	if err := writeJSON(s.monthPath(account, month), mf); err != nil {
		return err
	}
	_, err = s.recompute(account)
	return err
}

// Recompute rebuilds every month's opening/closing running balance and the
// account's cached balance from the transaction files, returning the balance.
func (s *Store) Recompute(account string) (Money, error) {
	unlock, err := s.lock()
	if err != nil {
		return 0, err
	}
	defer unlock()
	return s.recompute(account)
}

// recompute assumes the caller holds the lock.
func (s *Store) recompute(account string) (Money, error) {
	months, err := s.Months(account)
	if err != nil {
		return 0, err
	}
	var running Money
	for _, m := range months {
		mf, err := s.LoadMonth(account, m)
		if err != nil {
			return 0, err
		}
		mf.Opening = running
		var sum Money
		for _, t := range mf.Transactions {
			sum += t.Amount
		}
		running += sum
		mf.Closing = running
		if err := writeJSON(s.monthPath(account, m), mf); err != nil {
			return 0, err
		}
	}
	// Update the cached balance if the account is registered.
	accts, err := s.LoadAccounts()
	if err != nil {
		return 0, err
	}
	for i := range accts {
		if accts[i].Name == account {
			accts[i].Balance = running
			if err := s.SaveAccounts(accts); err != nil {
				return 0, err
			}
			break
		}
	}
	return running, nil
}

// --- locking & json ---

// lock takes a whole-ledger lock via an exclusive lockfile.
func (s *Store) lock() (func(), error) {
	if err := os.MkdirAll(s.Dir, 0o755); err != nil {
		return nil, err
	}
	path := filepath.Join(s.Dir, ".lock")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, fmt.Errorf("ledger is locked (%s); another duit process may be running, or remove it if stale", path)
		}
		return nil, err
	}
	f.Close()
	// ponytail: global lockfile; a crash leaves a stale lock needing manual
	// removal. Upgrade to flock/PID-check if that becomes a nuisance.
	return func() { os.Remove(path) }, nil
}

// validateSplits ensures a split transaction's parts sum to its total.
func validateSplits(t Transaction) error {
	if len(t.Splits) == 0 {
		return nil
	}
	var sum Money
	for _, s := range t.Splits {
		if s.Category == "" {
			return fmt.Errorf("split is missing a category")
		}
		sum += s.Amount
	}
	if sum != t.Amount {
		return fmt.Errorf("splits sum to %d but transaction amount is %d", sum, t.Amount)
	}
	return nil
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// writeJSON writes v as indented JSON atomically (temp file + rename) at 0600.
func writeJSON(path string, v any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, append(data, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
