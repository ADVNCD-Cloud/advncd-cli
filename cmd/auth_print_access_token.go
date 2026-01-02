package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
)

var authPrintAccessTokenCmd = &cobra.Command{
	Use:   "print-access-token",
	Short: "Print a valid access token (dev/debug)",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		fmt.Println(tb.AccessToken)
		return nil
	},
}