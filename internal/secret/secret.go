// Package secret stores the GitHub PAT in the OS keychain via go-keyring,
// keeping it out of the plaintext config file.
package secret

import (
	"errors"

	"github.com/zalando/go-keyring"
)

const service = "duit"

// keyFor maps a git remote to a keychain account. Empty remote uses "default"
// so multiple remotes can coexist under distinct accounts.
func keyFor(remote string) string {
	if remote == "" {
		return "default"
	}
	return remote
}

// Available reports whether a working OS keychain backend is present. It does a
// real Set+Delete of a probe key so a headless box with no Secret Service
// returns false instead of hanging. Any error means unavailable.
func Available() bool {
	const probe = "__duit_probe__"
	if err := keyring.Set(service, probe, "x"); err != nil {
		return false
	}
	return keyring.Delete(service, probe) == nil
}

// SetToken stores token for the given remote.
func SetToken(remote, token string) error {
	return keyring.Set(service, keyFor(remote), token)
}

// GetToken returns the stored token for remote, or ("", nil) if none is set.
func GetToken(remote string) (string, error) {
	tok, err := keyring.Get(service, keyFor(remote))
	if errors.Is(err, keyring.ErrNotFound) {
		return "", nil
	}
	return tok, err
}

// DeleteToken removes the token for remote. A missing token is not an error.
func DeleteToken(remote string) error {
	err := keyring.Delete(service, keyFor(remote))
	if errors.Is(err, keyring.ErrNotFound) {
		return nil
	}
	return err
}
