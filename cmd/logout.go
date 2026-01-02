package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Remove local credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := creds.DefaultStore()
		if err != nil {
			return err
		}
		if err := store.Delete(); err != nil {
			return err
		}
		fmt.Println("✓ Logged out (local credentials removed)")
		return nil
	},
}