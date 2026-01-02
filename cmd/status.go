package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
)

var statusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show local auth status",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		me, tb, err := auth.GetIdentity(ctx)
		if err != nil {
			return err
		}

		fmt.Println("auth: ok")
		fmt.Printf("email: %s\n", me.Email)
		fmt.Printf("token_expires_in: %s\n", time.Until(tb.Expiry).Truncate(time.Second))
		fmt.Printf("creds: %s\n", tb.CredsPath)
		return nil
	},
}