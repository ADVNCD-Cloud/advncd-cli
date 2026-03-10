package cmd

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/oauth"
)

var (
	loginClientIDFlag     string
	loginClientSecretFlag string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google Cloud (Authorization Code + PKCE)",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := creds.DefaultStore()
		if err != nil {
			return err
		}
		existing, err := store.Load()
		if err != nil {
			return err
		}

		clientID := firstNonEmpty(
			strings.TrimSpace(loginClientIDFlag),
			strings.TrimSpace(os.Getenv("ADVNCD_GCP_CLIENT_ID")),
		)
		clientSecret := firstNonEmpty(
			strings.TrimSpace(loginClientSecretFlag),
			strings.TrimSpace(os.Getenv("ADVNCD_GCP_CLIENT_SECRET")),
		)
		if existing != nil {
			clientID = firstNonEmpty(clientID, strings.TrimSpace(existing.ClientID))
			clientSecret = firstNonEmpty(clientSecret, strings.TrimSpace(existing.ClientSecret))
		}

		if clientID == "" {
			return apperr.New(apperr.AuthMissingClientID).
				WithFix("Provide OAuth client id via --client-id or ADVNCD_GCP_CLIENT_ID.").
				WithFix(`Example: advncd login --client-id "xxxx.apps.googleusercontent.com"`)
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

		expiry := time.Now().Add(time.Duration(tok.ExpiresIn) * time.Second)
		refreshToken := strings.TrimSpace(tok.RefreshToken)

		// Google may not always return refresh_token on re-consent.
		if refreshToken == "" && existing != nil &&
			strings.TrimSpace(existing.ClientID) == clientID &&
			strings.TrimSpace(existing.RefreshToken) != "" {
			refreshToken = strings.TrimSpace(existing.RefreshToken)
			fmt.Println("i Reusing existing refresh_token from local credentials.")
		}

		c := creds.Credentials{
			Version: 1,

			Email:  me.Email,
			Scopes: scopes,

			ClientID:     clientID,
			ClientSecret: clientSecret,

			AccessToken:  tok.AccessToken,
			RefreshToken: refreshToken,
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

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func init() {
	loginCmd.Flags().StringVar(&loginClientIDFlag, "client-id", "", "Google OAuth client ID")
	loginCmd.Flags().StringVar(&loginClientSecretFlag, "client-secret", "", "Google OAuth client secret (optional)")
}
