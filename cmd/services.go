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

var servicesCmd = &cobra.Command{
	Use:   "services",
	Short: "Cloud Run services (list, describe, metrics, open)",
	Args:  cobra.NoArgs,
	RunE: func(cmd *cobra.Command, args []string) error {
		// default action = list
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

		services, err := gcprun.ListServices(ctx, cr.AccessToken, cfg.ProjectID, cfg.Region)
		if err != nil {
			return err
		}

		if len(services) == 0 {
			fmt.Printf("No services found in %s.\n", cfg.Region)
			return nil
		}

		fmt.Printf("Services (project=%s, region=%s)\n", cfg.ProjectID, cfg.Region)
		fmt.Println("NAME\tSTATUS\tURL")

		for _, s := range services {
			url := s.URL
			if url == "" {
				url = "-"
			}
			fmt.Printf("%s\t%s\t%s\n", s.Name, s.Status, url)
		}

		return nil
	},
}