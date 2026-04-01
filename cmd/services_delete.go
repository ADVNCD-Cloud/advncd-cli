package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/registry"
)

var (
	servicesDeleteYes bool
)

var servicesDeleteCmd = &cobra.Command{
	Use:   "delete <service>",
	Short: "Delete a Cloud Run service",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		serviceName := strings.TrimSpace(args[0])
		if serviceName == "" {
			return fmt.Errorf("service name is required")
		}

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

		if !servicesDeleteYes {
			fmt.Printf("Delete service %q in project=%s region=%s?\n", serviceName, cfg.ProjectID, cfg.Region)
			fmt.Print("Type the service name to confirm: ")
			in := bufio.NewReader(os.Stdin)
			s, _ := in.ReadString('\n')
			s = strings.TrimSpace(s)
			if s != serviceName {
				fmt.Println("Aborted.")
				return nil
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		if err := gcprun.DeleteService(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, serviceName); err != nil {
			return err
		}
		if err := removeDeploymentRecordByServiceIdentity(contracts.ServiceIdentity{
			ServiceName: serviceName,
			ProjectID:   cfg.ProjectID,
			Region:      cfg.Region,
		}); err != nil {
			return err
		}

		fmt.Println("✓ Service delete request accepted.")
		fmt.Println("Cloud Run may take a short time to fully remove the service.")
		return nil
	},
}

func removeDeploymentRecordByServiceIdentity(identity contracts.ServiceIdentity) error {
	store, err := registry.DefaultStore()
	if err != nil {
		return err
	}
	rec, err := store.ResolveDeploymentByServiceIdentity(identity)
	if err != nil {
		return err
	}
	if rec == nil || strings.TrimSpace(rec.DeploymentID) == "" {
		return nil
	}
	return store.RemoveDeployment(rec.DeploymentID)
}

func init() {
	servicesDeleteCmd.Flags().BoolVar(&servicesDeleteYes, "yes", false, "Skip confirmation prompt")
}
