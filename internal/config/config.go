// Package config loads and saves duit's application config. It lives outside
// the data repo (at ~/.config/duit/config.json) so the auth token is never
// committed to the user's ledger repo.
package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config is duit's application configuration.
type Config struct {
	DataDir         string `json:"data_dir"`
	DefaultCurrency string `json:"default_currency"`
	Remote          string `json:"remote,omitempty"`
	Auth            Auth   `json:"auth,omitempty"`
}

// Auth describes how to authenticate git pushes to the user's remote.
type Auth struct {
	Method string `json:"method,omitempty"`  // "ssh" or "pat"
	SSHKey string `json:"ssh_key,omitempty"` // path to private key (ssh)
	Token  string `json:"token,omitempty"`   // GitHub PAT (pat); secret, 0600, not in repo
}

// DefaultPath returns the config file path (~/.config/duit/config.json).
func DefaultPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "duit", "config.json"), nil
}

// DefaultDataDir returns the suggested data directory (~/.local/share/duit).
func DefaultDataDir() (string, error) {
	if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		return filepath.Join(x, "duit"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".local", "share", "duit"), nil
}

// Load reads the config at path. A missing file returns os.ErrNotExist so
// callers can tell "not initialized yet" from a real error.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c Config
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	return &c, nil
}

// Save writes the config at path (parent dir 0700, file 0600 — it holds a token).
func Save(path string, c *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o600)
}
