package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/creds"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

var servicesDescribeCmd = &cobra.Command{
	Use:   "describe <service>",
	Short: "Describe a Cloud Run service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		name := args[0]

		cfgStore, err := config.DefaultStore()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		if cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
			return fmt.Errorf("missing project/region config; run: advncd init")
		}

		credStore, err := creds.DefaultStore()
		if err != nil {
			return err
		}
		cr, err := credStore.Load()
		if err != nil {
			return err
		}
		if cr == nil || cr.AccessToken == "" {
			return fmt.Errorf("not logged in; run: advncd login")
		}

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		svc, err := gcprun.GetService(ctx, cr.AccessToken, cfg.ProjectID, cfg.Region, name)
		if err != nil {
			return err
		}

		fmt.Printf("Service: %s\n", svc.Name)
		if svc.URL != "" {
			fmt.Printf("URL:     %s\n", svc.URL)
		} else {
			fmt.Printf("URL:     -\n")
		}
		if svc.Image != "" {
			fmt.Printf("Image:   %s\n", svc.Image)
		} else {
			fmt.Printf("Image:   -\n")
		}

		fmt.Println()
		fmt.Println("Conditions:")
		if len(svc.Conditions) == 0 {
			fmt.Println("  - (none)")
		} else {
			for _, c := range svc.Conditions {
				// Ready: CONDITION_SUCCEEDED / CONDITION_FAILED
				fmt.Printf("  - %s: %s\n", c.Type, c.State)
			}
		}

		return nil
	},
}