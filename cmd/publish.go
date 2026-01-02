package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/cloudbuild"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpartifact"
)

var (
	publishName string
)

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Build and deploy the current Go app to Cloud Run",
	RunE: func(cmd *cobra.Command, args []string) error {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
		defer cancel()

		// Auth (valid token)
		tb, err := auth.GetAccessToken(ctx)
		if err != nil {
			return err
		}

		// Config (project/region)
		cfgStore, err := config.DefaultStore()
		if err != nil {
			return err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return err
		}
		if cfg == nil || cfg.ProjectID == "" || cfg.Region == "" {
			fmt.Println("config: not set")
			fmt.Println("fix: run `advncd init`")
			return nil
		}

		// Project root = current directory (MVP)
		wd, err := os.Getwd()
		if err != nil {
			return err
		}

		// C0: require go.mod
		if _, err := os.Stat(filepath.Join(wd, "go.mod")); err != nil {
			fmt.Println("Not a Go module (go.mod not found in current directory).")
			fmt.Println("fix: run `advncd publish` from your Go project root (where go.mod is).")
			return nil
		}

		// Service name = folder slug by default, can override with --name
		svc := publishName
		if svc == "" {
			svc = projectslug.FromPathBase(wd)
		} else {
			svc = projectslug.Slugify(svc)
		}
		if svc == "" {
			fmt.Println("Unable to determine service name.")
			fmt.Println("fix: run `advncd publish --name <service>`")
			return nil
		}

		// Artifact Registry image
		// repo = advncd (MVP)
		repo := "advncd"
		image := fmt.Sprintf("%s-docker.pkg.dev/%s/%s/%s:latest", cfg.Region, cfg.ProjectID, repo, svc)

		fmt.Println("publish:")
		fmt.Printf("  project: %s\n", cfg.ProjectID)
		fmt.Printf("  region:  %s\n", cfg.Region)
		fmt.Printf("  service: %s\n", svc)
		fmt.Printf("  image:   %s\n", image)
		fmt.Println()

		// 1) Build & push container via Cloud Build (Buildpacks)
		fmt.Println("Ensuring Artifact Registry repo exists...")
		if err := gcpartifact.EnsureDockerRepo(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, "advncd"); err != nil {
			return err
		}
		fmt.Println("Building (Cloud Build + Buildpacks)...")
		build, err := cloudbuild.SubmitBuildpacksBuild(ctx, cloudbuild.SubmitRequest{
			AccessToken: tb.AccessToken,
			ProjectID:   cfg.ProjectID,
			SourceDir:   wd,
			Image:       image,
		})
		if err != nil {
			return err
		}

		fmt.Printf("✓ Build submitted: %s\n", build.ID)
		if build.LogURL != "" {
			fmt.Printf("  logs: %s\n", build.LogURL)
		}

		fmt.Println("Waiting for build to complete...")
		final, err := cloudbuild.WaitBuild(ctx, cloudbuild.WaitRequest{
			AccessToken: tb.AccessToken,
			ProjectID:   cfg.ProjectID,
			Region:      cfg.Region,
			BuildID:     build.ID,
			PollEvery:   3 * time.Second,
		})
		if err != nil {
			return err
		}

		if final.Status != "SUCCESS" {
			fmt.Println("Build did not succeed.")
			fmt.Printf("status: %s\n", final.Status)
			if final.LogURL != "" {
				fmt.Printf("logs: %s\n", final.LogURL)
			}
			fmt.Println("fix: open build logs and check buildpack detection / Go entrypoint.")
			fmt.Println("fix: ensure your app listens on $PORT (Cloud Run requirement).")
			fmt.Println("fix: ensure Artifact Registry repo exists: advncd")
			return nil
		}

		fmt.Println("✓ Build completed")

		// 2) Deploy to Cloud Run (create or update)
		fmt.Println("Deploying to Cloud Run...")
		deployed, err := gcprun.DeployService(ctx, gcprun.DeployRequest{
			AccessToken: tb.AccessToken,
			ProjectID:   cfg.ProjectID,
			Region:      cfg.Region,
			ServiceName: svc,
			Image:       image,
		})
		if err != nil {
			return err
		}

		fmt.Println("✓ Service deployed")
		fmt.Println("Allowing unauthenticated access...")
		if err := gcprun.AllowUnauthenticated(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, svc); err != nil {
			return err
		}
		fmt.Println("✓ Public access enabled")
		if deployed.URL != "" {
			fmt.Println()
			fmt.Printf("URL: %s\n", deployed.URL)
		} else {
			fmt.Println()
			fmt.Println("URL: (not returned)")
			fmt.Println("fix: open Cloud Run console to find the service URL.")
		}

		return nil
	},
}

func init() {
	publishCmd.Flags().StringVar(&publishName, "name", "", "Cloud Run service name (defaults to current folder name)")
}