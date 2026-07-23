package cli

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/RizkyChandra/duit/internal/config"
	"github.com/RizkyChandra/duit/internal/secret"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// resolveAuth returns the auth config with the PAT filled in. For method "pat"
// with no token in the config file, it pulls the token from the OS keychain.
func resolveAuth(c *config.Config) config.Auth {
	a := c.Auth
	if a.Method == "pat" && a.Token == "" {
		if tok, err := secret.GetToken(c.Remote); err == nil && tok != "" {
			a.Token = tok
		}
	}
	return a
}

// storeToken saves a PAT to the keychain when available, else into the config
// file (0600) as a fallback. It mutates c.Auth accordingly (Token stays empty
// when stored in the keychain) but does not persist c — the caller saves it.
func storeToken(c *config.Config, token string) error {
	c.Auth.Method = "pat"
	if secret.Available() {
		if err := secret.SetToken(c.Remote, token); err != nil {
			return err
		}
		c.Auth.Token = ""
		fmt.Println("Token stored in the OS keychain.")
		return nil
	}
	c.Auth.Token = token
	fmt.Fprintln(os.Stderr, "warning: no OS keychain available; token stored in config file (0600).")
	return nil
}

func saveConfig(c *config.Config) error {
	p, err := config.DefaultPath()
	if err != nil {
		return err
	}
	return config.Save(p, c)
}

// readSecret reads a secret. On a real terminal it hides input; when stdin is
// piped (scripts/CI) it reads a plain line from r so automation still works.
func readSecret(r *bufio.Reader, label string) (string, error) {
	fmt.Print(label)
	fd := int(os.Stdin.Fd())
	if term.IsTerminal(fd) {
		b, err := term.ReadPassword(fd)
		fmt.Println()
		return strings.TrimSpace(string(b)), err
	}
	line, err := r.ReadString('\n')
	return strings.TrimSpace(line), err
}

func authCmd() *cobra.Command {
	cmd := &cobra.Command{Use: "auth", Short: "Manage the GitHub token (keychain-backed)"}
	cmd.AddCommand(authSetTokenCmd(), authMigrateCmd())
	return cmd
}

func authSetTokenCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "set-token",
		Aliases: []string{"relogin"},
		Short:   "Set/replace the GitHub PAT (stored in the OS keychain when available)",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := mustCtx()
			if err != nil {
				return err
			}
			tok, err := readSecret(bufio.NewReader(os.Stdin), "GitHub token (input hidden): ")
			if err != nil {
				return err
			}
			if tok == "" {
				return fmt.Errorf("empty token")
			}
			if err := storeToken(c, tok); err != nil {
				return err
			}
			return saveConfig(c)
		},
	}
}

func authMigrateCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Move a plaintext token from the config file into the OS keychain",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := mustCtx()
			if err != nil {
				return err
			}
			if c.Auth.Token == "" {
				return fmt.Errorf("no plaintext token in config to migrate")
			}
			if !secret.Available() {
				return fmt.Errorf("no OS keychain available on this system")
			}
			if err := secret.SetToken(c.Remote, c.Auth.Token); err != nil {
				return err
			}
			c.Auth.Token = ""
			if err := saveConfig(c); err != nil {
				return err
			}
			fmt.Println("Migrated token into the OS keychain and cleared it from the config file.")
			return nil
		},
	}
}
