package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/oauth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/ui"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google Cloud using OAuth Device Flow",
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := os.Getenv("ADVNCD_GCP_CLIENT_ID")
		if clientID == "" {
			return apperr.New(apperr.ErrMissingClientID).
				WithFix("Create an OAuth Client ID (Desktop app) in Google Cloud Console, then export ADVNCD_GCP_CLIENT_ID.").
				WithFix(`Example: export ADVNCD_GCP_CLIENT_ID="xxxx.apps.googleusercontent.com"`)
		}

		// Minimal scopes for v0 (userinfo + cloud-platform)
		scopes := []string{
			"openid",
			"email",
			"profile",
			"https://www.googleapis.com/auth/cloud-platform",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		resp, err := oauth.StartDeviceFlow(ctx, oauth.DeviceFlowRequest{
			ClientID: clientID,
			Scopes:   scopes,
		})
		if err != nil {
			return err
		}

		// GitHub CLI style output
		ui.PrintLoginInstructions(ui.LoginInstructions{
			VerificationURL: resp.VerificationURL,
			UserCode:        resp.UserCode,
			ExpiresIn:       resp.ExpiresIn,
			Interval:        resp.Interval,
		})

		// For now (A1) we stop here.
		// Next step (A2) will poll token endpoint and persist tokens.
		fmt.Println()
		fmt.Println("After you authorize in the browser, we'll add token polling (next step).")
		return nil
	},
}