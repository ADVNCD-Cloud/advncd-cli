package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpcrm"
)

var (
	projectsDeleteYes bool
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "Manage GCP projects (list/delete)",
}

var projectsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List ACTIVE GCP projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		projects, err := gcpcrm.ListProjects(ctx, tb.AccessToken)
		if err != nil {
			return err
		}
		if len(projects) == 0 {
			fmt.Println("No ACTIVE projects found.")
			return nil
		}

		sort.Slice(projects, func(i, j int) bool {
			return projects[i].ProjectID < projects[j].ProjectID
		})

		fmt.Println("ACTIVE projects:")
		for _, p := range projects {
			if strings.TrimSpace(p.Name) != "" && p.Name != p.ProjectID {
				fmt.Printf("  - %s (%s)\n", p.ProjectID, p.Name)
			} else {
				fmt.Printf("  - %s\n", p.ProjectID)
			}
		}
		fmt.Println()
		fmt.Println("Delete one project:")
		fmt.Println("  advncd projects delete <project_id>")
		return nil
	},
}

var projectsDeleteCmd = &cobra.Command{
	Use:   "delete <project_id>",
	Short: "Delete a GCP project (moves it to deletion lifecycle)",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		projectID := strings.TrimSpace(args[0])
		if projectID == "" {
			return fmt.Errorf("project_id is required")
		}

		if !projectsDeleteYes {
			fmt.Printf("Delete project %q?\n", projectID)
			fmt.Print("Type the project id to confirm: ")
			in := bufio.NewReader(os.Stdin)
			s, _ := in.ReadString('\n')
			s = strings.TrimSpace(s)
			if s != projectID {
				fmt.Println("Aborted.")
				return nil
			}
		}

		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
		defer cancel()

		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		if err := gcpcrm.DeleteProject(ctx, tb.AccessToken, projectID); err != nil {
			return err
		}

		fmt.Println("✓ Delete request accepted.")
		fmt.Println("Project enters deletion lifecycle in Google Cloud.")
		return nil
	},
}

func init() {
	projectsDeleteCmd.Flags().BoolVar(&projectsDeleteYes, "yes", false, "Skip confirmation prompt")
	projectsCmd.AddCommand(projectsListCmd)
	projectsCmd.AddCommand(projectsDeleteCmd)
}
