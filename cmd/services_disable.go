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
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
)

var (
	servicesDisableYes  bool
	servicesDisableHard bool
)

func runServiceDisable(serviceName string, yes bool, hard bool) error {
	serviceName = strings.TrimSpace(serviceName)
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

	if !yes {
		fmt.Printf("Disable service %q in project=%s region=%s?\n", serviceName, cfg.ProjectID, cfg.Region)
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

	if hard {
		if err := gcprun.DeleteService(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, serviceName); err != nil {
			return err
		}
		fmt.Println("✓ Kill switch (hard): service delete requested.")
		return nil
	}

	if err := gcprun.DenyUnauthenticated(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, serviceName); err != nil {
		return err
	}
	fmt.Println("✓ Service disabled for public traffic (allUsers invoker removed).")
	fmt.Println("Use `advncd services open <service>` to verify access behavior.")
	return nil
}

var servicesDisableCmd = &cobra.Command{
	Use:   "disable <service>",
	Short: "Disable a service quickly (kill switch)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		return runServiceDisable(args[0], servicesDisableYes, servicesDisableHard)
	},
}

func init() {
	servicesDisableCmd.Flags().BoolVar(&servicesDisableYes, "yes", false, "Skip confirmation prompt")
	servicesDisableCmd.Flags().BoolVar(&servicesDisableHard, "hard", false, "Hard kill switch: delete service instead of removing public access")
}
