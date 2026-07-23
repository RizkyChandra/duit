package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func attachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "attach <id> <file>",
		Short: "Attach a receipt file to a transaction",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			id, srcPath := args[0], expandHome(args[1])
			account, month, _, err := store.FindTransaction(id)
			if err != nil {
				return err
			}
			rel := filepath.Join("attachments", account, id+filepath.Ext(srcPath))
			dst := filepath.Join(c.DataDir, rel)
			if err := copyFile(srcPath, dst); err != nil {
				return err
			}
			if err := store.SetAttachment(account, month, id, rel); err != nil {
				return err
			}
			commit(c, "attach receipt to "+id)
			fmt.Printf("Attached %s to transaction %s (%s)\n", rel, id, account)
			return nil
		},
	}
}

func receiptCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "receipt <id>",
		Short: "Print the path of a transaction's attached receipt",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			c, store, err := mustCtx()
			if err != nil {
				return err
			}
			_, _, tx, err := store.FindTransaction(args[0])
			if err != nil {
				return err
			}
			if tx.Attachment == "" {
				return fmt.Errorf("transaction %s has no attachment", args[0])
			}
			fmt.Println(filepath.Join(c.DataDir, tx.Attachment))
			return nil
		},
	}
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
		return err
	}
	// Receipts are financial documents — keep them private like the JSON files.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := io.Copy(out, in); err != nil {
		return err
	}
	return out.Close()
}
