package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/oauth"
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google Cloud (Authorization Code + PKCE)",
	RunE: func(cmd *cobra.Command, args []string) error {
		clientID := os.Getenv("ADVNCD_GCP_CLIENT_ID")
		clientSecret := os.Getenv("ADVNCD_GCP_CLIENT_SECRET")

		if clientID == "" {
			return apperr.New(apperr.AuthMissingClientID).
				WithFix("For now (dev), export ADVNCD_GCP_CLIENT_ID from your OAuth Desktop Client ID.").
				WithFix(`Example: export ADVNCD_GCP_CLIENT_ID="xxxx.apps.googleusercontent.com"`)
		}

		scopes := []string{
			"openid",
			"email",
			"profile",
			"https://www.googleapis.com/auth/cloud-platform",
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		fmt.Println("Starting local callback server...")
		sess, err := oauth.BeginAuthCodePKCE(oauth.AuthCodeRequest{
			ClientID: clientID,
			Scopes:   scopes,
		})
		if err != nil {
			return err
		}

		fmt.Println("Opening browser for authentication...")
		if !openBrowser(sess.AuthURL) {
			fmt.Println("Could not open browser automatically. Please open this URL:")
			fmt.Printf("  %s\n", sess.AuthURL)
		}

		fmt.Println("Waiting for authentication to complete in browser...")
		result, err := sess.Wait(ctx)
		if err != nil {
			return err
		}

		fmt.Println("Exchanging authorization code for tokens...")
		tok, err := oauth.ExchangeAuthCode(
			ctx,
			clientID,
			clientSecret,
			result.Code,
			result.RedirectURI,
			result.CodeVerifier,
		)
		if err != nil {
			return err
		}

		fmt.Println("Fetching user info...")
		me, err := oauth.FetchUserInfo(ctx, tok.AccessToken)
		if err != nil {
			return err
		}

		fmt.Println()
		if me.Email != "" {
			fmt.Printf("✓ Logged in as %s\n", me.Email)
		} else {
			fmt.Println("✓ Logged in")
		}

		// ---- A3: persist creds locally ----
		store, err := creds.DefaultStore()
		if err != nil {
			return err
		}

		expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)

		c := creds.Credentials{
			Version: 1,

			Email:  me.Email,
			Scopes: scopes,

			ClientID: clientID,

			AccessToken:  tok.AccessToken,
			RefreshToken: tok.RefreshToken,
			Expiry:       expiry,
			TokenType:    tok.TokenType,
		}

		if c.RefreshToken == "" {
			// Not fatal, but important for real “local-first” experience
			fmt.Println("! Warning: refresh_token is empty.")
			fmt.Println("  This can happen if Google doesn't re-issue refresh tokens on repeated consents.")
			fmt.Println("  If future commands fail after token expiry, run: advncd login")
		}

		if err := store.Save(c); err != nil {
			return err
		}

		fmt.Printf("✓ Saved credentials: %s\n", store.Path)
		// ---- end A3 ----

		return nil
	},
}

func openBrowser(url string) bool {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	if err := cmd.Start(); err != nil {
		return false
	}
	return true
}