package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/advncdyaml"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/detect"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpcrm"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
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

		wd, err := os.Getwd()
		if err == nil {
			written, yamlPath, yamlErr := ensureAdvncdYAML(wd, cfg.ProjectID, cfg.Region)
			if yamlErr != nil {
				fmt.Printf("! Failed to scaffold %s: %v\n", advncdyaml.FileName, yamlErr)
			} else if written {
				fmt.Printf("✓ Created %s\n", yamlPath)
				detected, detErr := detect.Project(wd)
				if detErr != nil {
					fmt.Printf("! Detect after init failed: %v\n", detErr)
				} else {
					_, writeErr := writeDetectedYAML(wd, nil, detected.Profile, detected.ServiceProposal, cfg.ProjectID, cfg.Region)
					if writeErr != nil {
						fmt.Printf("! Failed to apply detected profile to %s: %v\n", yamlPath, writeErr)
					} else {
						fmt.Printf("✓ Applied detected runtime/build/port to %s\n", yamlPath)
						fmt.Printf("i Detect result: runtime=%s build=%s port=%d confidence=%s\n",
							detected.Profile.Runtime, detected.Profile.BuildStrategy, detected.Profile.Port, detected.Profile.Confidence)
					}
				}
			} else {
				fmt.Printf("i Kept existing %s\n", yamlPath)
			}
		}
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

func ensureAdvncdYAML(rootDir, projectID, region string) (written bool, path string, err error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return false, "", fmt.Errorf("root directory is required")
	}
	path = filepath.Join(rootDir, advncdyaml.FileName)

	if _, statErr := os.Stat(path); statErr == nil {
		return false, path, nil
	} else if !os.IsNotExist(statErr) {
		return false, path, statErr
	}

	serviceName := projectslug.Slugify(filepath.Base(rootDir))
	if serviceName == "" {
		serviceName = "app"
	}

	content := fmt.Sprintf(`version: 1

service:
  name: %s
  port: 8080

deploy:
  project: %s
  region: %s
  allow_service_rename: false

env:
  required: []
  optional: []

guardrails:
  deployment_profile: default
  cloud_run:
    min_instances: 0
    max_instances: 1
    timeout_seconds: 30
    memory: 256Mi
  webhook:
    require_auth: true
    auth_mode: header
    secret_header: X-Webhook-Secret
    reject_query_secrets: true
    idempotency_enabled: true
    idempotency_ttl_seconds: 3600
    rate_limit_per_minute: 120
  budget:
    enabled: false
    amount_eur: 10
    thresholds_csv: 0.5,0.9,1.0

# Optional explicit overrides (uncomment only when needed):
# build:
#   strategy: buildpacks
#
# runtime:
#   family: <runtime-family>
#   framework: <framework>
`, serviceName, strings.TrimSpace(projectID), strings.TrimSpace(region))

	if writeErr := os.WriteFile(path, []byte(content), 0o644); writeErr != nil {
		return false, path, writeErr
	}
	return true, path, nil
}
