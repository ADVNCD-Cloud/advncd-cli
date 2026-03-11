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
	"github.com/ADVNCD-Cloud/advncd-cli/internal/authbroker"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
)

var (
	loginAuthBaseURLFlag string
)

var loginCmd = &cobra.Command{
	Use:   "login",
	Short: "Authenticate with Google Cloud via auth broker backend",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := creds.DefaultStore()
		if err != nil {
			return err
		}
		existing, err := store.Load()
		if err != nil {
			return err
		}

		baseURL := resolveAuthBaseURL(existing, loginAuthBaseURLFlag)
		if strings.TrimSpace(baseURL) == "" {
			return apperr.New(apperr.AuthBrokerProtocol).
				WithFix("Set ADVNCD_AUTH_BASE_URL or use --auth-base-url.")
		}

		broker := authbroker.New(baseURL)

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Minute)
		defer cancel()

		deviceName, _ := os.Hostname()
		if strings.TrimSpace(deviceName) == "" {
			deviceName = runtime.GOOS
		}

		fmt.Println("Requesting login session from auth broker...")
		start, err := broker.Start(ctx, deviceName)
		if err != nil {
			return err
		}

		fmt.Println("Opening browser for authentication...")
		if !openBrowser(start.VerifyURL) {
			fmt.Println("Could not open browser automatically. Please open this URL:")
			fmt.Printf("  %s\n", start.VerifyURL)
		}

		fmt.Println("Waiting for authentication to complete in browser...")

		expiresIn := start.ExpiresIn
		if expiresIn <= 0 {
			expiresIn = 600
		}
		deadline := time.Now().Add(time.Duration(expiresIn) * time.Second)
		interval := time.Duration(start.IntervalSeconds) * time.Second
		if interval < time.Second {
			interval = 5 * time.Second
		}

		for {
			if time.Now().After(deadline) {
				return apperr.New(apperr.AuthPollTimeout).
					WithFix("Login timed out waiting for approval. Run 'advncd login' again.")
			}

			poll, err := broker.Poll(ctx, start.SessionID)
			if err != nil {
				return err
			}

			switch strings.TrimSpace(poll.Status) {
			case "", "pending":
				if poll.RetryAfterSeconds > 0 {
					interval = time.Duration(poll.RetryAfterSeconds) * time.Second
				}
				time.Sleep(interval)
				continue
			case "denied":
				return apperr.New(apperr.AuthPollDenied).
					WithFix("Approve login in browser and retry 'advncd login'.")
			case "expired":
				return apperr.New(apperr.AuthPollExpired).
					WithFix("Run 'advncd login' again to start a new session.")
			case "approved":
				if poll.ExpiresIn <= 0 {
					poll.ExpiresIn = 900
				}
				if strings.TrimSpace(poll.AppAccessToken) == "" || strings.TrimSpace(poll.AppRefreshToken) == "" {
					return apperr.New(apperr.AuthBrokerProtocol).
						WithFix("Auth broker approved login but returned empty tokens.")
				}

				c := creds.Credentials{
					Version: 2,
					Email:   strings.TrimSpace(poll.Email),

					AuthBaseURL: baseURL,

					AppAccessToken:      strings.TrimSpace(poll.AppAccessToken),
					AppRefreshToken:     strings.TrimSpace(poll.AppRefreshToken),
					AppAccessTokenExpiry: time.Now().Add(time.Duration(poll.ExpiresIn) * time.Second),
				}

				if err := store.Save(c); err != nil {
					return err
				}

				email := c.Email
				if email == "" {
					email = "<unknown>"
				}
				fmt.Println()
				fmt.Printf("✓ Logged in as %s\n", email)
				fmt.Printf("✓ Saved credentials: %s\n", store.Path)
				return nil
			default:
				if poll.RetryAfterSeconds > 0 {
					interval = time.Duration(poll.RetryAfterSeconds) * time.Second
				}
				return apperr.New(apperr.AuthBrokerProtocol).
					WithMeta("poll_status", poll.Status).
					WithFix("Auth broker returned unknown poll status.")
			}
		}
	},
}

func resolveAuthBaseURL(existing *creds.Credentials, flagValue string) string {
	stored := ""
	if existing != nil {
		stored = existing.AuthBaseURL
	}
	return firstNonEmpty(
		flagValue,
		os.Getenv("ADVNCD_AUTH_BASE_URL"),
		stored,
		authbroker.DefaultBaseURL,
	)
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
	loginCmd.Flags().StringVar(&loginAuthBaseURLFlag, "auth-base-url", "", "Auth broker base URL (defaults to ADVNCD_AUTH_BASE_URL or built-in URL)")
}
