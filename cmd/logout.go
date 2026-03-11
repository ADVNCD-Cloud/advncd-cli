package cmd

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/authbroker"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
)

var logoutCmd = &cobra.Command{
	Use:   "logout",
	Short: "Revoke broker session and remove local credentials",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := creds.DefaultStore()
		if err != nil {
			return err
		}

		existing, err := store.Load()
		if err != nil {
			return err
		}

		if existing != nil && strings.TrimSpace(existing.AppRefreshToken) != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
			defer cancel()

			baseURL := auth.ResolveAuthBaseURL(existing.AuthBaseURL)
			broker := authbroker.New(baseURL)
			if err := broker.Logout(ctx, existing.AppRefreshToken); err != nil {
				return err
			}
		}

		if err := store.Delete(); err != nil {
			return err
		}
		fmt.Println("✓ Logged out (local credentials removed)")
		return nil
	},
}
