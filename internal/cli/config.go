package cli

import (
	"fmt"

	"github.com/RizkyChandra/duit/internal/config"
	"github.com/spf13/cobra"
)

func configCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Show the current configuration (token redacted)",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, _, err := mustCtx()
			if err != nil {
				return err
			}
			path, _ := config.DefaultPath()
			token := ""
			switch {
			case c.Auth.Method != "pat":
			case c.Auth.Token != "":
				token = "(in config file)"
			default:
				token = "(in OS keychain or unset)"
			}
			view := struct {
				ConfigPath      string `json:"config_path"`
				DataDir         string `json:"data_dir"`
				DefaultCurrency string `json:"default_currency"`
				Remote          string `json:"remote,omitempty"`
				AuthMethod      string `json:"auth_method,omitempty"`
				Token           string `json:"token,omitempty"`
			}{path, c.DataDir, c.DefaultCurrency, c.Remote, c.Auth.Method, token}

			if jsonOut {
				return printJSON(view)
			}
			w := tw()
			fmt.Fprintf(w, "config path\t%s\n", view.ConfigPath)
			fmt.Fprintf(w, "data dir\t%s\n", view.DataDir)
			fmt.Fprintf(w, "default currency\t%s\n", view.DefaultCurrency)
			if view.Remote != "" {
				fmt.Fprintf(w, "remote\t%s\n", view.Remote)
			}
			if view.AuthMethod != "" {
				fmt.Fprintf(w, "auth\t%s %s\n", view.AuthMethod, view.Token)
			}
			return w.Flush()
		},
	}
}
