package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/cloudbuild"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpartifact"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/publishplan"
)

var (
	publishName      string
	publishEnvFile   string
	publishEnv       []string
	publishPlanPath  string
	publishScanForce bool
	detectPath       string
	detectName       string
	detectWriteYAML  bool
	deployPath       string
	deployName       string
)

var publishCmd = &cobra.Command{
	Use:   "publish",
	Short: "Build and deploy the current app to Cloud Run",
	RunE:  runPublishDeploy,
}

var publishDeployCmd = &cobra.Command{
	Use:   "deploy",
	Short: "Deploy app to Cloud Run using deploy YAML",
	RunE:  runPublishDeploy,
}

var publishScanCmd = &cobra.Command{
	Use:   "scan",
	Short: "Scan project and generate deploy YAML",
	RunE:  runPublishScan,
}

var deployCmd = &cobra.Command{
	Use:   contracts.CommandDeploy,
	Short: "Deploy app to Cloud Run",
	Args:  cobra.NoArgs,
	RunE:  runPrimaryDeploy,
}

var detectCmd = &cobra.Command{
	Use:   contracts.CommandDetect,
	Short: "Detect deploy profile for a project",
	Args:  cobra.NoArgs,
	RunE:  runPrimaryDetect,
}

func init() {
	publishCmd.PersistentFlags().StringVar(&publishPlanPath, "plan", publishplan.DefaultFileName, "Path to deploy plan YAML")
	publishCmd.PersistentFlags().StringVar(&publishName, "name", "", "Cloud Run service name override")
	publishCmd.PersistentFlags().StringVar(&publishEnvFile, "env-file", "", "Path to dotenv file (deploy runtime override; scan writes it to plan env_file)")
	publishCmd.PersistentFlags().StringArrayVar(&publishEnv, "env", nil, "Runtime env var override/addition (KEY=VALUE), repeatable")

	publishScanCmd.Flags().BoolVar(&publishScanForce, "force", false, "Overwrite existing plan file")

	publishCmd.AddCommand(publishDeployCmd)
	publishCmd.AddCommand(publishScanCmd)

	deployCmd.Flags().StringVar(&deployPath, "path", ".", "Project path to deploy")
	deployCmd.Flags().StringVar(&deployName, "name", "", "Cloud Run service name override")

	detectCmd.Flags().StringVar(&detectPath, "path", ".", "Project path to detect")
	detectCmd.Flags().StringVar(&detectName, "name", "", "Service name override for proposal output")
	detectCmd.Flags().BoolVar(&detectWriteYAML, "write-yaml", false, "Write detected runtime/build/port values into advncd.yaml")
}

func runPublishScan(cmd *cobra.Command, args []string) error {
	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	planPath := resolvePlanPath(wd, publishPlanPath)
	if !publishScanForce {
		if _, err := os.Stat(planPath); err == nil {
			return fmt.Errorf("plan file already exists: %s (use --force to overwrite)", planPath)
		}
	}

	plan, err := publishplan.Scan(wd)
	if err != nil {
		return err
	}
	if strings.TrimSpace(publishName) != "" {
		plan.Service = strings.TrimSpace(publishName)
	}
	if strings.TrimSpace(publishEnvFile) != "" {
		plan.EnvFile = strings.TrimSpace(publishEnvFile)
	}
	plan, err = publishplan.Normalize(plan)
	if err != nil {
		return err
	}

	if err := publishplan.WriteFile(planPath, plan); err != nil {
		return err
	}

	fmt.Printf("✓ Deploy plan written: %s\n", planPath)
	fmt.Printf("  stack: %s\n", plan.Stack)
	fmt.Printf("  service: %s\n", plan.Service)
	fmt.Printf("  port: %d\n", plan.Port)
	if plan.EnvFile != "" {
		fmt.Printf("  env_file: %s\n", plan.EnvFile)
	}
	return nil
}

func runPublishDeploy(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		return err
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
		fmt.Println("config: not set")
		fmt.Println("fix: run `advncd init`")
		return nil
	}

	wd, err := os.Getwd()
	if err != nil {
		return err
	}

	planPath := resolvePlanPath(wd, publishPlanPath)
	plan, err := loadOrCreatePlanWithWizard(wd, planPath)
	if err != nil {
		return err
	}

	plan, err = applyPublishOverrides(plan)
	if err != nil {
		return err
	}

	sourceDir := plan.SourceDir
	if !filepath.IsAbs(sourceDir) {
		sourceDir = filepath.Join(wd, sourceDir)
	}
	if st, err := os.Stat(sourceDir); err != nil || !st.IsDir() {
		return fmt.Errorf("source_dir does not exist or is not a directory: %s", sourceDir)
	}

	envFromFileAndArgs, err := buildPublishEnv(plan.EnvFile, publishEnv)
	if err != nil {
		return err
	}
	envVars := mergeStringMap(plan.Env, envFromFileAndArgs)
	policy := resolveGuardrailsPolicy(plan.Service, nil)
	if err := validateWebhookAndSecretPatterns(envVars, policy); err != nil {
		return err
	}
	if err := validateWebhookProtectionEnv(plan.Service, envVars, policy); err != nil {
		return err
	}
	plainEnv, secretEnv, err := syncSensitiveEnvToSecretManager(ctx, tb.AccessToken, cfg.ProjectID, plan.Service, envVars)
	if err != nil {
		return err
	}

	repo := plan.ImageRepo
	image := buildPublishImage(cfg.Region, cfg.ProjectID, repo, plan.Service, time.Now())

	fmt.Println("publish:")
	fmt.Printf("  project: %s\n", cfg.ProjectID)
	fmt.Printf("  region:  %s\n", cfg.Region)
	fmt.Printf("  service: %s\n", plan.Service)
	fmt.Printf("  stack:   %s\n", plan.Stack)
	fmt.Printf("  source:  %s\n", sourceDir)
	fmt.Printf("  image:   %s\n", image)
	if len(envVars) > 0 {
		fmt.Printf("  env:     %d vars\n", len(envVars))
	}
	fmt.Println()

	fmt.Printf("Ensuring Artifact Registry repo %q exists...\n", repo)
	if err := gcpartifact.EnsureDockerRepo(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, repo); err != nil {
		return err
	}

	switch plan.BuildMethod {
	case publishplan.BuildMethodBuildpacks:
		fmt.Println("Building (Cloud Build + Buildpacks)...")
	default:
		return fmt.Errorf("unsupported build_method %q", plan.BuildMethod)
	}

	build, err := cloudbuild.SubmitBuildpacksBuild(ctx, cloudbuild.SubmitRequest{
		AccessToken: tb.AccessToken,
		ProjectID:   cfg.ProjectID,
		SourceDir:   sourceDir,
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
		fmt.Println("fix: open build logs and check buildpack detection / start command.")
		fmt.Println("fix: ensure your app listens on $PORT (Cloud Run requirement).")
		fmt.Printf("fix: ensure Artifact Registry repo exists: %s\n", repo)
		return nil
	}

	fmt.Println("✓ Build completed")
	fmt.Println("Deploying to Cloud Run...")

	deployReq := gcprun.DeployRequest{
		AccessToken:   tb.AccessToken,
		ProjectID:     cfg.ProjectID,
		Region:        cfg.Region,
		ServiceName:   plan.Service,
		Image:         image,
		Env:           plainEnv,
		SecretEnv:     secretEnv,
		ContainerPort: plan.Port,
		Memory:        plan.Memory,
		MinInstances:  plan.MinInstances,
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
	if plan.AllowUnauthenticated {
		fmt.Println("Allowing unauthenticated access...")
		if err := gcprun.AllowUnauthenticated(ctx, tb.AccessToken, cfg.ProjectID, cfg.Region, plan.Service); err != nil {
			return err
		}
		fmt.Println("✓ Public access enabled")
	}

	fmt.Println()
	if deployed.URL != "" {
		fmt.Printf("URL: %s\n", deployed.URL)
	} else {
		fmt.Println("URL: (not returned)")
		fmt.Println("fix: open Cloud Run console to find the service URL.")
	}

	return nil
}

func loadOrCreatePlanWithWizard(wd, planPath string) (publishplan.Plan, error) {
	plan, err := publishplan.ReadFile(planPath)
	if err == nil {
		return plan, nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return publishplan.Plan{}, err
	}

	fmt.Printf("Deploy plan not found: %s\n", planPath)
	ok, err := promptYesNo("Run setup wizard to create it now?", true)
	if err != nil {
		return publishplan.Plan{}, err
	}
	if !ok {
		return publishplan.Plan{}, fmt.Errorf("deploy cancelled: plan file is required")
	}

	scanned, scanErr := publishplan.Scan(wd)
	if scanErr != nil {
		scanned = publishplan.NewDefaults(projectslug.FromPathBase(wd))
	}

	wizardPlan, err := runPublishWizard(scanned)
	if err != nil {
		return publishplan.Plan{}, err
	}
	if err := publishplan.WriteFile(planPath, wizardPlan); err != nil {
		return publishplan.Plan{}, err
	}
	fmt.Printf("✓ Deploy plan created: %s\n", planPath)
	return wizardPlan, nil
}

func runPublishWizard(seed publishplan.Plan) (publishplan.Plan, error) {
	service, err := promptDefaultLine("Service name", seed.Service)
	if err != nil {
		return publishplan.Plan{}, err
	}

	stack, err := promptDefaultLine(
		fmt.Sprintf("Stack (%s)", strings.Join(publishplan.SupportedStacks(), "/")),
		seed.Stack,
	)
	if err != nil {
		return publishplan.Plan{}, err
	}
	stack = strings.ToLower(strings.TrimSpace(stack))
	if stack == "" {
		stack = publishplan.StackUnknown
	}

	defaultPort, defaultEnv := publishplan.StackDefaults(stack)
	portDefault := seed.Port
	if portDefault <= 0 {
		portDefault = defaultPort
	}
	portStr, err := promptDefaultLine("Container port", strconv.Itoa(portDefault))
	if err != nil {
		return publishplan.Plan{}, err
	}
	port, err := strconv.Atoi(strings.TrimSpace(portStr))
	if err != nil {
		return publishplan.Plan{}, fmt.Errorf("invalid port %q", portStr)
	}

	memory, err := promptDefaultLine("Memory limit (blank to keep default)", seed.Memory)
	if err != nil {
		return publishplan.Plan{}, err
	}

	envFile, err := promptDefaultLine("env_file path (blank to skip)", seed.EnvFile)
	if err != nil {
		return publishplan.Plan{}, err
	}

	allowPublic, err := promptYesNo("Allow unauthenticated access?", true)
	if err != nil {
		return publishplan.Plan{}, err
	}

	minInstances, err := promptOptionalInt("Min instances (-1 to skip)", seed.MinInstances)
	if err != nil {
		return publishplan.Plan{}, err
	}

	env := mergeStringMap(defaultEnv, seed.Env)
	for {
		pair, err := promptLine("Extra env KEY=VALUE (empty to finish)")
		if err != nil {
			return publishplan.Plan{}, err
		}
		pair = strings.TrimSpace(pair)
		if pair == "" {
			break
		}
		i := strings.Index(pair, "=")
		if i <= 0 {
			fmt.Println("Invalid format. Use KEY=VALUE.")
			continue
		}
		k := strings.TrimSpace(pair[:i])
		v := pair[i+1:]
		if k == "" {
			fmt.Println("Invalid format. Key cannot be empty.")
			continue
		}
		env[k] = v
	}

	plan := publishplan.Plan{
		Version:              publishplan.Version,
		Service:              service,
		Stack:                stack,
		SourceDir:            seed.SourceDir,
		BuildMethod:          publishplan.BuildMethodBuildpacks,
		ImageRepo:            seed.ImageRepo,
		Port:                 port,
		Memory:               strings.TrimSpace(memory),
		MinInstances:         minInstances,
		AllowUnauthenticated: allowPublic,
		EnvFile:              strings.TrimSpace(envFile),
		Env:                  env,
	}

	return publishplan.Normalize(plan)
}

func applyPublishOverrides(plan publishplan.Plan) (publishplan.Plan, error) {
	out := plan
	if strings.TrimSpace(publishName) != "" {
		out.Service = strings.TrimSpace(publishName)
	}
	if strings.TrimSpace(publishEnvFile) != "" {
		out.EnvFile = strings.TrimSpace(publishEnvFile)
	}
	return publishplan.Normalize(out)
}

func promptDefaultLine(question, def string) (string, error) {
	def = strings.TrimSpace(def)
	if def == "" {
		return promptLine(question)
	}
	s, err := promptLine(fmt.Sprintf("%s [%s]", question, def))
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(s) == "" {
		return def, nil
	}
	return strings.TrimSpace(s), nil
}

func promptOptionalInt(question string, def *int) (*int, error) {
	defText := "-1"
	if def != nil {
		defText = strconv.Itoa(*def)
	}
	s, err := promptDefaultLine(question, defText)
	if err != nil {
		return nil, err
	}
	n, err := strconv.Atoi(strings.TrimSpace(s))
	if err != nil {
		return nil, fmt.Errorf("invalid number %q", s)
	}
	if n < 0 {
		return nil, nil
	}
	return &n, nil
}

func buildPublishEnv(envFile string, envArgs []string) (map[string]string, error) {
	merged := map[string]string{}

	if envFile != "" {
		fileEnv, err := godotenv.Read(envFile)
		if err != nil {
			if os.IsNotExist(err) {
				return nil, fmt.Errorf("env-file not found: %s", envFile)
			}
			return nil, fmt.Errorf("failed to read env-file %q: %w", envFile, err)
		}
		for k, v := range fileEnv {
			if strings.TrimSpace(k) == "" {
				return nil, fmt.Errorf("env-file %q contains an empty key", envFile)
			}
			merged[k] = v
		}
	}

	for _, pair := range envArgs {
		i := strings.Index(pair, "=")
		if i < 0 {
			return nil, fmt.Errorf("invalid --env %q: expected KEY=VALUE", pair)
		}
		key := pair[:i]
		val := pair[i+1:]
		if strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid --env %q: key cannot be empty", pair)
		}
		merged[key] = val
	}

	return merged, nil
}

func buildPublishImage(region, projectID, repo, service string, now time.Time) string {
	deployTag := now.UTC().Format("20060102-150405")
	return fmt.Sprintf("%s-docker.pkg.dev/%s/%s/%s:%s", region, projectID, repo, service, deployTag)
}

func resolvePlanPath(wd, planFlag string) string {
	path := strings.TrimSpace(planFlag)
	if path == "" {
		path = publishplan.DefaultFileName
	}
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(wd, path)
}

func mergeStringMap(base, overlay map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range base {
		out[k] = v
	}
	for k, v := range overlay {
		out[k] = v
	}
	return out
}
