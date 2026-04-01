package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/advncdyaml"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/cloudbuild"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/detect"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpartifact"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/registry"
)

func runPrimaryDetect(cmd *cobra.Command, args []string) error {
	root, err := resolveProjectPath(detectPath)
	if err != nil {
		return err
	}

	detected, yamlCfg, serviceName, err := detectWithOverrides(root, detectName)
	if err != nil {
		return err
	}

	fmt.Printf("path: %s\n", root)
	if yamlCfg != nil {
		fmt.Printf("advncd.yaml: %s\n", filepath.Join(root, advncdyaml.FileName))
	} else {
		fmt.Println("advncd.yaml: (not found)")
	}
	fmt.Printf("runtime: %s\n", detected.Runtime)
	fmt.Printf("build strategy: %s\n", detected.BuildStrategy)
	fmt.Printf("port: %d\n", detected.Port)
	fmt.Printf("confidence: %s\n", detected.Confidence)
	if len(detected.Warnings) == 0 {
		fmt.Println("warnings: none")
	} else {
		fmt.Println("warnings:")
		for _, w := range detected.Warnings {
			fmt.Printf("  - %s\n", w)
		}
	}
	fmt.Printf("service name proposal: %s\n", serviceName)
	if detectWriteYAML {
		projectID, region := "", ""
		if yamlCfg == nil || strings.TrimSpace(yamlCfg.Deploy.Project) == "" || strings.TrimSpace(yamlCfg.Deploy.Region) == "" {
			projectID, region = resolveConfigTargetBestEffort()
		}
		updatedPath, err := writeDetectedYAML(root, yamlCfg, detected, serviceName, projectID, region)
		if err != nil {
			return err
		}
		fmt.Printf("✓ Updated %s from detected profile\n", updatedPath)
	}
	return nil
}

func runPrimaryDeploy(cmd *cobra.Command, args []string) error {
	root, err := resolveProjectPath(deployPath)
	if err != nil {
		return err
	}

	profile, yamlCfg, serviceName, err := detectWithOverrides(root, deployName)
	if err != nil {
		return err
	}

	projectID, region, err := resolveDeployTarget(yamlCfg)
	if err != nil {
		return err
	}
	policy := resolveGuardrailsPolicy(serviceName, yamlCfg)
	if err := validateWebhookProtectionConfig(serviceName, yamlCfg, policy); err != nil {
		return err
	}

	printDeployPlan(root, projectID, region, serviceName, profile)
	if profile.Confidence != "high" {
		ok, err := promptYesNo("Detection confidence is not high. Continue deploy?", false)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("deploy cancelled")
		}
	}

	if profile.BuildStrategy != "buildpacks" {
		return fmt.Errorf("unsupported build strategy %q in deploy flow", profile.BuildStrategy)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		return err
	}

	repo := "advncd"
	image := buildPublishImage(region, projectID, repo, serviceName, time.Now())

	fmt.Printf("Ensuring Artifact Registry repo %q exists...\n", repo)
	if err := gcpartifact.EnsureDockerRepo(ctx, tb.AccessToken, projectID, region, repo); err != nil {
		return err
	}

	fmt.Println("Building (Cloud Build + Buildpacks)...")
	build, err := cloudbuild.SubmitBuildpacksBuild(ctx, cloudbuild.SubmitRequest{
		AccessToken: tb.AccessToken,
		ProjectID:   projectID,
		SourceDir:   root,
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
		ProjectID:   projectID,
		Region:      region,
		BuildID:     build.ID,
		PollEvery:   3 * time.Second,
	})
	if err != nil {
		return err
	}
	if final.Status != "SUCCESS" {
		fmt.Printf("Build did not succeed (status=%s).\n", final.Status)
		if final.LogURL != "" {
			fmt.Printf("logs: %s\n", final.LogURL)
		}
		return nil
	}

	fmt.Println("✓ Build completed")
	fmt.Println("Deploying to Cloud Run...")
	deployReq := gcprun.DeployRequest{
		AccessToken:   tb.AccessToken,
		ProjectID:     projectID,
		Region:        region,
		ServiceName:   serviceName,
		Image:         image,
		ContainerPort: profile.Port,
	}
	deployReq = applyCloudRunGuardrails(deployReq, policy)
	for _, w := range collectCloudRunGuardrailWarnings(deployReq, policy) {
		fmt.Printf("warning: %s\n", w)
	}
	deployed, err := gcprun.DeployService(ctx, deployReq)
	if err != nil {
		return err
	}

	fmt.Println("✓ Service deployed")
	fmt.Println("Allowing unauthenticated access...")
	if err := gcprun.AllowUnauthenticated(ctx, tb.AccessToken, projectID, region, serviceName); err != nil {
		return err
	}
	fmt.Println("✓ Public access enabled")

	status := "unknown"
	if svc, err := gcprun.GetService(ctx, tb.AccessToken, projectID, region, serviceName); err == nil && svc != nil {
		status = strings.ToLower(gcprun.DeriveStatus(svc.Conditions))
	}

	if err := writeDeployRecord(contracts.DeploymentRecord{
		SourceType:         contracts.SourceTypeCode,
		SourcePathOptional: root,
		ServiceName:        serviceName,
		ProjectID:          projectID,
		Region:             region,
		URL:                strings.TrimSpace(deployed.URL),
		Status:             status,
	}); err != nil {
		return err
	}

	if deployed.URL != "" {
		fmt.Printf("URL: %s\n", deployed.URL)
	}
	return nil
}

func resolveProjectPath(pathFlag string) (string, error) {
	p := strings.TrimSpace(pathFlag)
	if p == "" {
		p = "."
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("path is not a directory: %s", abs)
	}
	return abs, nil
}

func detectWithOverrides(root, nameOverride string) (contracts.DeployProfile, *advncdyaml.Config, string, error) {
	detected, err := detect.Project(root)
	if err != nil {
		return contracts.DeployProfile{}, nil, "", err
	}

	yamlCfg, err := advncdyaml.ReadFromRoot(root)
	if err != nil {
		return contracts.DeployProfile{}, nil, "", err
	}

	profile := applyYAMLOverrides(detected.Profile, yamlCfg)
	profile = normalizeProfileAfterOverrides(profile)
	serviceName := resolveServiceName(nameOverride, yamlCfg, detected.ServiceProposal)
	return profile, yamlCfg, serviceName, nil
}

func applyYAMLOverrides(profile contracts.DeployProfile, yamlCfg *advncdyaml.Config) contracts.DeployProfile {
	if yamlCfg == nil {
		return profile
	}
	runtimeFamily := strings.TrimSpace(strings.ToLower(yamlCfg.Runtime.Family))
	if runtimeFamily != "" && runtimeFamily != "unknown" {
		profile.Runtime = runtimeFamily
	}
	if strings.TrimSpace(yamlCfg.Runtime.Framework) != "" {
		profile.Framework = strings.TrimSpace(strings.ToLower(yamlCfg.Runtime.Framework))
	}
	if strings.TrimSpace(yamlCfg.Build.Strategy) != "" {
		profile.BuildStrategy = strings.TrimSpace(strings.ToLower(yamlCfg.Build.Strategy))
	}
	if strings.TrimSpace(yamlCfg.Build.StartCommand) != "" {
		profile.StartStrategy = "explicit"
	}
	if yamlCfg.Service.Port > 0 {
		profile.Port = yamlCfg.Service.Port
	}
	if len(yamlCfg.Env.Required) > 0 {
		profile.Signals = append(profile.Signals, "advncd.yaml:env.required")
	}
	if len(yamlCfg.Env.Optional) > 0 {
		profile.Signals = append(profile.Signals, "advncd.yaml:env.optional")
	}
	return profile
}

func normalizeProfileAfterOverrides(profile contracts.DeployProfile) contracts.DeployProfile {
	if strings.TrimSpace(profile.Runtime) == "" {
		profile.Runtime = "unknown"
	}
	if strings.TrimSpace(profile.BuildStrategy) == "" {
		profile.BuildStrategy = "manual"
	}
	if profile.Port <= 0 {
		profile.Port = 8080
		profile.Warnings = append(profile.Warnings, "using default port 8080")
	}
	if profile.Runtime == "unknown" || profile.BuildStrategy == "manual" {
		profile.Confidence = "low"
		if !containsString(profile.Warnings, "runtime detection is uncertain") {
			profile.Warnings = append(profile.Warnings, "runtime detection is uncertain")
		}
	}
	return profile
}

func containsString(items []string, needle string) bool {
	for _, v := range items {
		if v == needle {
			return true
		}
	}
	return false
}

func resolveServiceName(nameOverride string, yamlCfg *advncdyaml.Config, proposal string) string {
	service := strings.TrimSpace(nameOverride)
	if service == "" && yamlCfg != nil {
		service = strings.TrimSpace(yamlCfg.Service.Name)
	}
	if service == "" {
		service = strings.TrimSpace(proposal)
	}
	service = projectslug.Slugify(service)
	if service == "" {
		return "app"
	}
	return service
}

func resolveDeployTarget(yamlCfg *advncdyaml.Config) (string, string, error) {
	cfgStore, err := config.DefaultStore()
	if err != nil {
		return "", "", err
	}
	cfg, err := cfgStore.Load()
	if err != nil {
		return "", "", err
	}

	projectID := ""
	region := ""
	if cfg != nil {
		projectID = strings.TrimSpace(cfg.ProjectID)
		region = strings.TrimSpace(cfg.Region)
	}
	if yamlCfg != nil {
		if strings.TrimSpace(yamlCfg.Deploy.Project) != "" {
			projectID = strings.TrimSpace(yamlCfg.Deploy.Project)
		}
		if strings.TrimSpace(yamlCfg.Deploy.Region) != "" {
			region = strings.TrimSpace(yamlCfg.Deploy.Region)
		}
	}
	if projectID == "" || region == "" {
		return "", "", fmt.Errorf("missing project/region config; run `advncd init` or set deploy.project/deploy.region in advncd.yaml")
	}
	return projectID, region, nil
}

func printDeployPlan(root, projectID, region, serviceName string, profile contracts.DeployProfile) {
	fmt.Println("deploy plan:")
	fmt.Printf("  path: %s\n", root)
	fmt.Printf("  project: %s\n", projectID)
	fmt.Printf("  region: %s\n", region)
	fmt.Printf("  service: %s\n", serviceName)
	fmt.Printf("  runtime: %s\n", profile.Runtime)
	fmt.Printf("  build strategy: %s\n", profile.BuildStrategy)
	fmt.Printf("  port: %d\n", profile.Port)
	fmt.Printf("  confidence: %s\n", profile.Confidence)
	if len(profile.Warnings) > 0 {
		fmt.Println("  warnings:")
		for _, w := range profile.Warnings {
			fmt.Printf("    - %s\n", w)
		}
	}
	fmt.Println()
}

func writeDeployRecord(rec contracts.DeploymentRecord) error {
	store, err := registry.DefaultStore()
	if err != nil {
		return err
	}
	return store.UpsertDeploymentByServiceIdentity(rec)
}

func writeDetectedYAML(root string, current *advncdyaml.Config, profile contracts.DeployProfile, serviceName, projectID, region string) (string, error) {
	cfg := &advncdyaml.Config{}
	if current != nil {
		*cfg = *current
	}
	if cfg.Version == 0 {
		cfg.Version = 1
	}
	if strings.TrimSpace(serviceName) != "" {
		cfg.Service.Name = serviceName
	}
	if profile.Port > 0 {
		cfg.Service.Port = profile.Port
	}
	if strings.TrimSpace(projectID) != "" {
		cfg.Deploy.Project = projectID
	}
	if strings.TrimSpace(region) != "" {
		cfg.Deploy.Region = region
	}
	cfg.Build.Strategy = profile.BuildStrategy
	cfg.Runtime.Family = profile.Runtime
	cfg.Runtime.Framework = profile.Framework

	policy := resolveGuardrailsPolicy(serviceName, cfg)
	cfg.Guardrails.DeploymentProfile = policy.DeploymentProfile
	cfg.Guardrails.CloudRun.MinInstances = policy.CloudRun.MinInstances
	cfg.Guardrails.CloudRun.MaxInstances = policy.CloudRun.MaxInstances
	cfg.Guardrails.CloudRun.TimeoutSeconds = policy.CloudRun.TimeoutSeconds
	cfg.Guardrails.CloudRun.Memory = policy.CloudRun.Memory
	cfg.Guardrails.Webhook.RequireAuth = policy.Webhook.RequireAuth
	cfg.Guardrails.Webhook.AuthMode = policy.Webhook.AuthMode
	cfg.Guardrails.Webhook.SecretHeader = policy.Webhook.HeaderName
	cfg.Guardrails.Webhook.RejectQuerySecrets = policy.Webhook.RejectQuerySecrets
	cfg.Guardrails.Webhook.IdempotencyEnabled = policy.Webhook.IdempotencyEnabled
	cfg.Guardrails.Webhook.IdempotencyTTLSec = policy.Webhook.IdempotencyTTLSec
	cfg.Guardrails.Webhook.RateLimitPerMinute = policy.Webhook.RateLimitPerMinute
	cfg.Guardrails.Budget.Enabled = policy.Budget.Enabled
	cfg.Guardrails.Budget.AmountEUR = policy.Budget.AmountEUR
	if len(policy.Budget.Thresholds) > 0 {
		parts := make([]string, 0, len(policy.Budget.Thresholds))
		for _, v := range policy.Budget.Thresholds {
			parts = append(parts, fmt.Sprintf("%.2f", v))
		}
		cfg.Guardrails.Budget.ThresholdsCSV = strings.Join(parts, ",")
	}

	path, err := advncdyaml.WriteToRoot(root, cfg)
	if err != nil {
		return "", err
	}
	return path, nil
}

func resolveConfigTargetBestEffort() (projectID, region string) {
	store, err := config.DefaultStore()
	if err != nil {
		return "", ""
	}
	cfg, err := store.Load()
	if err != nil || cfg == nil {
		return "", ""
	}
	return strings.TrimSpace(cfg.ProjectID), strings.TrimSpace(cfg.Region)
}
