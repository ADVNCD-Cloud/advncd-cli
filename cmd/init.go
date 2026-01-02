package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpcrm"
)

var (
	initProject string
	initRegion  string
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Select default GCP project and region",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		// ensure logged in + get valid token
		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		projectID := strings.TrimSpace(initProject)
		region := strings.TrimSpace(initRegion)

		// If project not provided, list projects and ask user to pick
		if projectID == "" {
			fmt.Println("Loading GCP projects...")
			projects, err := gcpcrm.ListProjects(ctx, tb.AccessToken)
			if err != nil {
				return err
			}
			if len(projects) == 0 {
				fmt.Println("No ACTIVE projects found for this account.")
				fmt.Println("You can still set a project manually:")
				fmt.Println("  advncd init --project <project_id> --region <region>")
				return nil
			}

			sort.Slice(projects, func(i, j int) bool {
				return projects[i].ProjectID < projects[j].ProjectID
			})

			fmt.Println()
			fmt.Println("Select GCP project:")
			max := len(projects)
			if max > 30 {
				max = 30
				fmt.Println("(showing first 30; use --project to set manually if needed)")
			}
			for i := 0; i < max; i++ {
				p := projects[i]
				label := p.ProjectID
				if strings.TrimSpace(p.Name) != "" && p.Name != p.ProjectID {
					label = fmt.Sprintf("%s (%s)", p.ProjectID, p.Name)
				}
				fmt.Printf("  [%d] %s\n", i+1, label)
			}

			choice, err := readChoice(1, max)
			if err != nil {
				return err
			}
			projectID = projects[choice-1].ProjectID
		}

		// Region: if not provided, ask
		if region == "" {
			region = readRegion()
		}

		store, err := config.DefaultStore()
		if err != nil {
			return err
		}

		cfg := config.Config{
			Version:   1,
			ProjectID: projectID,
			Region:    region,
		}

		if err := store.Save(cfg); err != nil {
			return err
		}

		fmt.Println()
		fmt.Printf("✓ Project set: %s\n", cfg.ProjectID)
		fmt.Printf("✓ Region set:  %s\n", cfg.Region)
		fmt.Printf("✓ Saved config: %s\n", store.Path)
		return nil
	},
}

func init() {
	initCmd.Flags().StringVar(&initProject, "project", "", "GCP project id (optional, skips interactive selection)")
	initCmd.Flags().StringVar(&initRegion, "region", "", "Default region (e.g. europe-west1)")
}

func readChoice(min, max int) (int, error) {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Printf("Enter choice [%d-%d]: ", min, max)
		s, _ := in.ReadString('\n')
		s = strings.TrimSpace(s)
		n, err := strconv.Atoi(s)
		if err != nil || n < min || n > max {
			fmt.Println("Invalid choice.")
			continue
		}
		return n, nil
	}
}

func readRegion() string {
	in := bufio.NewReader(os.Stdin)

	type region struct {
		ID   string
		Desc string
	}

	common := []region{
		{"europe-west1", "Belgium"},
		{"europe-west3", "Frankfurt"},
		{"europe-west4", "Netherlands"},
		{"europe-west6", "Zurich"},
		{"us-central1", "Iowa"},
		{"us-east1", "South Carolina"},
		{"us-west1", "Oregon"},
		{"asia-northeast1", "Tokyo"},
		{"asia-southeast1", "Singapore"},
	}

	fmt.Println()
	fmt.Println("Select region:")
	for i, r := range common {
		fmt.Printf("  [%d] %s (%s)\n", i+1, r.ID, r.Desc)
	}
	fmt.Printf("  [%d] %s\n", len(common)+1, "Enter custom region")

	for {
		fmt.Printf("Enter choice [1-%d]: ", len(common)+1)
		s, _ := in.ReadString('\n')
		s = strings.TrimSpace(s)
		n, err := strconv.Atoi(s)
		if err != nil || n < 1 || n > len(common)+1 {
			fmt.Println("Invalid choice.")
			continue
		}
		if n <= len(common) {
			return common[n-1].ID
		}
		fmt.Print("Enter region (e.g. europe-west1): ")
		r, _ := in.ReadString('\n')
		r = strings.TrimSpace(r)
		if r == "" {
			fmt.Println("Region cannot be empty.")
			continue
		}
		return r
	}
}