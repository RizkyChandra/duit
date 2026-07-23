package cli

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/RizkyChandra/duit/internal/config"
	"github.com/RizkyChandra/duit/internal/gitsync"
	"github.com/spf13/cobra"
	"golang.org/x/term"
)

func initCmd() *cobra.Command {
	var dataDir, currency, remote, auth, sshKey string
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create the ledger data dir, git repo, and config",
		RunE: func(cmd *cobra.Command, args []string) error {
			r := bufio.NewReader(os.Stdin)
			if dataDir == "" {
				def, _ := config.DefaultDataDir()
				dataDir = prompt(r, "Data directory", def)
			}
			abs, err := filepath.Abs(expandHome(dataDir))
			if err != nil {
				return err
			}
			dataDir = abs
			if currency == "" {
				currency = prompt(r, "Default currency", "USD")
			}
			currency = strings.ToUpper(currency)
			if remote == "" {
				remote = prompt(r, "Git remote URL (optional)", "")
			}

			var a config.Auth
			if remote != "" {
				if auth == "" {
					auth = prompt(r, "Auth method (ssh/pat)", "ssh")
				}
				a.Method = strings.ToLower(auth)
				switch a.Method {
				case "pat":
					fmt.Print("GitHub token (input hidden): ")
					b, err := term.ReadPassword(int(os.Stdin.Fd()))
					fmt.Println()
					if err != nil {
						return err
					}
					a.Token = strings.TrimSpace(string(b))
				case "ssh":
					a.SSHKey = expandHome(prompt(r, "SSH private key path (empty = ssh-agent)", sshKey))
				default:
					return fmt.Errorf("unknown auth method %q (want ssh or pat)", a.Method)
				}
			}

			if err := os.MkdirAll(dataDir, 0o755); err != nil {
				return err
			}
			// Keep the lockfile and temp files out of git.
			if err := os.WriteFile(filepath.Join(dataDir, ".gitignore"), []byte(".lock\n*.tmp\n"), 0o644); err != nil {
				return err
			}
			if err := gitsync.EnsureRepo(dataDir, remote); err != nil {
				return err
			}

			c := &config.Config{DataDir: dataDir, DefaultCurrency: currency, Remote: remote, Auth: a}
			p, err := config.DefaultPath()
			if err != nil {
				return err
			}
			if err := config.Save(p, c); err != nil {
				return err
			}
			if _, err := gitsync.CommitAll(dataDir, "init duit ledger"); err != nil {
				fmt.Fprintln(os.Stderr, "warning: initial commit failed:", err)
			}
			fmt.Printf("Initialized duit ledger at %s (default currency %s)\n", dataDir, currency)
			fmt.Printf("Config: %s\nNext: duit account add <name>\n", p)
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&dataDir, "data-dir", "", "ledger data directory")
	f.StringVar(&currency, "currency", "", "default currency code (e.g. USD, IDR)")
	f.StringVar(&remote, "remote", "", "git remote URL")
	f.StringVar(&auth, "auth", "", "auth method: ssh or pat")
	f.StringVar(&sshKey, "ssh-key", "", "SSH private key path (ssh auth)")
	return cmd
}

func prompt(r *bufio.Reader, label, def string) string {
	if def != "" {
		fmt.Printf("%s [%s]: ", label, def)
	} else {
		fmt.Printf("%s: ", label)
	}
	line, _ := r.ReadString('\n')
	if line = strings.TrimSpace(line); line != "" {
		return line
	}
	return def
}
