package cmd

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/auth"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/config"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpcrm"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcpserviceusage"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
)

var (
	n8nDefaultImage  = "n8nio/n8n:1.86.0"
	n8nProject       string
	n8nProjectName   string
	n8nCreateProject bool
	n8nRegion        string
	n8nServiceName   string
	n8nImage         string
	n8nMemory        string
	n8nPort          int
	n8nMinInstances  int
	n8nNoCPUThrottle bool
	n8nRedeploy      bool
	n8nSetDefault    bool
	n8nPublicURL     string
	n8nDBURL         string
	n8nDBSchema      string
	n8nDBSSLReject   bool
	n8nEncryptKey    string
	projectIDPattern = regexp.MustCompile(`^[a-z][a-z0-9-]{4,28}[a-z0-9]$`)
)

var n8nCmd = &cobra.Command{
	Use:   "n8n",
	Short: "Deploy or redeploy n8n to Cloud Run",
	RunE:  runN8N,
}

var launchCmd = &cobra.Command{
	Use:   contracts.CommandLaunch + " [preset]",
	Short: "Launch a ready-made service (currently: n8n)",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runLaunch,
}

func init() {
	bindN8NFlags(n8nCmd)
	bindN8NFlags(launchCmd)
}

func bindN8NFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&n8nProject, "project", "", "GCP project ID to use")
	cmd.Flags().StringVar(&n8nProjectName, "project-name", "", "Display name for new project")
	cmd.Flags().BoolVar(&n8nCreateProject, "create-project", false, "Create project from --project")
	cmd.Flags().StringVar(&n8nRegion, "region", "", "Cloud Run region (default: interactive)")
	cmd.Flags().StringVar(&n8nServiceName, "service", "n8n", "Cloud Run service name")
	cmd.Flags().StringVar(&n8nImage, "image", n8nDefaultImage, "n8n container image")
	cmd.Flags().StringVar(&n8nMemory, "memory", "2Gi", "Memory limit for Cloud Run service")
	cmd.Flags().IntVar(&n8nPort, "port", 5678, "Container port for n8n")
	cmd.Flags().IntVar(&n8nMinInstances, "min-instances", 1, "Cloud Run minimum instances (-1 to keep current setting)")
	cmd.Flags().BoolVar(&n8nNoCPUThrottle, "no-cpu-throttling", true, "Disable CPU throttling for more stable n8n runtime")
	cmd.Flags().BoolVar(&n8nRedeploy, "redeploy", false, "Use project/region from saved config and redeploy existing service")
	cmd.Flags().BoolVar(&n8nSetDefault, "set-default", false, "Save selected project/region as default config")
	cmd.Flags().StringVar(&n8nPublicURL, "public-url", "", "Public base URL for n8n (auto-detected from service URL if empty)")
	cmd.Flags().StringVar(&n8nDBURL, "db-url", "", "PostgreSQL URL for persistent n8n state (e.g. Supabase)")
	cmd.Flags().StringVar(&n8nDBSchema, "db-schema", "public", "PostgreSQL schema for n8n tables")
	cmd.Flags().BoolVar(&n8nDBSSLReject, "db-ssl-reject-unauthorized", false, "Validate DB SSL certificate (for Supabase usually false)")
	cmd.Flags().StringVar(&n8nEncryptKey, "encryption-key", "", "N8N_ENCRYPTION_KEY (recommended: stable secret)")
}

func runLaunch(cmd *cobra.Command, args []string) error {
	if len(args) > 0 {
		if strings.ToLower(strings.TrimSpace(args[0])) != "n8n" {
			return fmt.Errorf("unsupported preset %q (available: n8n)", args[0])
		}
	}
	return runN8N(cmd, nil)
}

func runN8N(cmd *cobra.Command, args []string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	tb, err := auth.GetAccessToken(ctx)
	if err != nil {
		return err
	}

	projectID, region, err := resolveN8NTarget(ctx, tb.AccessToken)
	if err != nil {
		return err
	}

	proj, err := waitProjectActive(ctx, tb.AccessToken, projectID)
	if err != nil {
		return err
	}

	if err := ensureRunAPI(ctx, tb.AccessToken, proj.ProjectNumber); err != nil {
		return err
	}

	service := projectslug.Slugify(strings.TrimSpace(n8nServiceName))
	if service == "" {
		service = "n8n"
	}
	image := strings.TrimSpace(n8nImage)
	if image == "" {
		image = n8nDefaultImage
	}

	var existingSvc *gcprun.ServiceDetail
	publicURL := strings.TrimSpace(n8nPublicURL)
	svc, err := gcprun.GetService(ctx, tb.AccessToken, projectID, region, service)
	if err == nil {
		existingSvc = svc
		if publicURL == "" && strings.TrimSpace(svc.URL) != "" {
			publicURL = strings.TrimSpace(svc.URL)
		}
	} else if !isServiceNotFound(err) {
		return err
	}

	existingEnv := map[string]string{}
	if existingSvc != nil {
		existingEnv = copyStringMap(existingSvc.Env)
	}

	dbEnv, err := resolveN8NDatabaseEnv(existingEnv)
	if err != nil {
		return err
	}
	key, err := resolveN8NEncryptionKey(existingEnv)
	if err != nil {
		return err
	}

	env := mergeEnv(existingEnv, buildN8NEnv(publicURL, n8nPort))
	env = mergeEnv(env, dbEnv)
	if strings.TrimSpace(key) != "" {
		env["N8N_ENCRYPTION_KEY"] = strings.TrimSpace(key)
	}

	deployReq := gcprun.DeployRequest{
		AccessToken:   tb.AccessToken,
		ProjectID:     projectID,
		Region:        region,
		ServiceName:   service,
		Image:         image,
		ContainerPort: n8nPort,
		Memory:        strings.TrimSpace(n8nMemory),
		Env:           env,
	}
	if n8nMinInstances >= 0 {
		v := n8nMinInstances
		deployReq.MinInstances = &v
	}
	if n8nNoCPUThrottle {
		v := false // cpuIdle=false => no CPU throttling
		deployReq.CPUIDle = &v
	}

	fmt.Println("Deploying n8n to Cloud Run...")
	deployed, err := gcprun.DeployService(ctx, deployReq)
	if err != nil {
		return err
	}

	// On first deploy we only know final service URL after creation.
	finalURL := firstNonEmpty(strings.TrimSpace(n8nPublicURL), strings.TrimSpace(deployed.URL), strings.TrimSpace(publicURL))
	finalEnv := mergeEnv(existingEnv, buildN8NEnv(finalURL, n8nPort))
	finalEnv = mergeEnv(finalEnv, dbEnv)
	if strings.TrimSpace(key) != "" {
		finalEnv["N8N_ENCRYPTION_KEY"] = strings.TrimSpace(key)
	}
	if finalURL != "" && !sameStringMap(env, finalEnv) {
		fmt.Println("Applying n8n public URL settings...")
		deployReq.Env = finalEnv
		deployed2, err := gcprun.DeployService(ctx, deployReq)
		if err != nil {
			return err
		}
		if strings.TrimSpace(deployed2.URL) != "" {
			deployed = deployed2
		}
	}

	fmt.Println("Allowing unauthenticated access...")
	if err := gcprun.AllowUnauthenticated(ctx, tb.AccessToken, projectID, region, service); err != nil {
		return err
	}

	fmt.Println("✓ n8n deployed")
	if deployed.URL != "" {
		fmt.Printf("URL: %s\n", deployed.URL)
	} else {
		fmt.Println("URL: not returned by API; open Cloud Run console to copy it.")
	}
	return nil
}

func pickOrCreateProjectForN8N(ctx context.Context, accessToken string) (string, error) {
	projectID := strings.TrimSpace(n8nProject)
	if projectID != "" {
		if !projectIDPattern.MatchString(projectID) {
			return "", fmt.Errorf("invalid project id %q", projectID)
		}
		if n8nCreateProject {
			createdID, err := createProjectWithAutoSuffix(ctx, accessToken, projectID, strings.TrimSpace(n8nProjectName))
			if err != nil {
				return "", err
			}
			projectID = createdID
		}
		return projectID, nil
	}

	projects, err := gcpcrm.ListProjects(ctx, accessToken)
	if err != nil {
		return "", err
	}
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].ProjectID < projects[j].ProjectID
	})

	if len(projects) == 0 {
		fmt.Println("No ACTIVE projects found. Creating a new project is required.")
		return promptAndCreateProject(ctx, accessToken)
	}

	fmt.Println("Select project for n8n:")
	max := len(projects)
	if max > 30 {
		max = 30
		fmt.Println("(showing first 30)")
	}
	for i := 0; i < max; i++ {
		p := projects[i]
		label := p.ProjectID
		if strings.TrimSpace(p.Name) != "" && p.Name != p.ProjectID {
			label = fmt.Sprintf("%s (%s)", p.ProjectID, p.Name)
		}
		fmt.Printf("  [%d] %s\n", i+1, label)
	}
	createChoice := max + 1
	fmt.Printf("  [%d] Create new project\n", createChoice)

	choice, err := readChoice(1, createChoice)
	if err != nil {
		return "", err
	}
	if choice == createChoice {
		return promptAndCreateProject(ctx, accessToken)
	}
	return projects[choice-1].ProjectID, nil
}

func promptAndCreateProject(ctx context.Context, accessToken string) (string, error) {
	in := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Enter new project id (e.g. my-n8n-prod-123): ")
		pid, _ := in.ReadString('\n')
		pid = strings.TrimSpace(pid)
		if !projectIDPattern.MatchString(pid) {
			fmt.Println("Invalid project id format.")
			continue
		}

		fmt.Print("Enter project name (optional): ")
		name, _ := in.ReadString('\n')
		name = strings.TrimSpace(name)

		createdID, err := createProjectWithAutoSuffix(ctx, accessToken, pid, name)
		if err != nil {
			return "", err
		}
		return createdID, nil
	}
}

func waitProjectActive(ctx context.Context, accessToken, projectID string) (*gcpcrm.ProjectGet, error) {
	deadline := time.Now().Add(4 * time.Minute)
	var lastErr error

	for time.Now().Before(deadline) {
		p, err := gcpcrm.GetProject(ctx, accessToken, projectID)
		if err == nil && p != nil && p.ProjectNumber != "" && p.LifecycleState == "ACTIVE" {
			return p, nil
		}
		if err == nil && p != nil && p.ProjectNumber != "" && p.LifecycleState != "" {
			lastErr = fmt.Errorf("project lifecycle is %s (waiting for ACTIVE)", p.LifecycleState)
		} else if err != nil {
			lastErr = err
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(3 * time.Second):
		}
	}

	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("timed out waiting for project %s to become ACTIVE", projectID)
}

func ensureRunAPI(ctx context.Context, accessToken, projectNumber string) error {
	const runAPI = "run.googleapis.com"

	state, err := gcpserviceusage.GetServiceState(ctx, accessToken, projectNumber, runAPI)
	if err == nil && state == "ENABLED" {
		fmt.Println("✓ Cloud Run API is already enabled")
		return nil
	}

	fmt.Println("Enabling Cloud Run API...")
	if err := gcpserviceusage.EnableService(ctx, accessToken, projectNumber, runAPI); err != nil {
		return err
	}
	fmt.Println("✓ Cloud Run API enabled")
	return nil
}

func resolveN8NTarget(ctx context.Context, accessToken string) (string, string, error) {
	if n8nRedeploy {
		cfgStore, err := config.DefaultStore()
		if err != nil {
			return "", "", err
		}
		cfg, err := cfgStore.Load()
		if err != nil {
			return "", "", err
		}
		if cfg == nil {
			return "", "", fmt.Errorf("no saved config found; run `advncd init` or `advncd n8n` first")
		}

		projectID := firstNonEmpty(strings.TrimSpace(n8nProject), strings.TrimSpace(cfg.ProjectID))
		region := firstNonEmpty(strings.TrimSpace(n8nRegion), strings.TrimSpace(cfg.Region))
		if projectID == "" || region == "" {
			return "", "", fmt.Errorf("missing project/region for redeploy; provide flags or run `advncd init`")
		}
		fmt.Printf("Redeploy mode: project=%s region=%s\n", projectID, region)
		return projectID, region, nil
	}

	projectID, err := pickOrCreateProjectForN8N(ctx, accessToken)
	if err != nil {
		return "", "", err
	}

	region := strings.TrimSpace(n8nRegion)
	if region == "" {
		region = readRegion()
	}

	if n8nSetDefault {
		cfgStore, err := config.DefaultStore()
		if err != nil {
			return "", "", err
		}
		cfg := config.Config{
			Version:   1,
			ProjectID: projectID,
			Region:    region,
		}
		if err := cfgStore.Save(cfg); err != nil {
			return "", "", err
		}
		fmt.Printf("✓ Saved config: project=%s region=%s\n", projectID, region)
	} else {
		fmt.Printf("i Using project=%s region=%s for this run (default config unchanged)\n", projectID, region)
		fmt.Println("  Tip: add --set-default to persist this as default.")
	}

	return projectID, region, nil
}

func buildN8NEnv(publicURL string, port int) map[string]string {
	out := map[string]string{
		"N8N_ENDPOINT_HEALTH": "/healthz",
		"N8N_PORT":            strconv.Itoa(port),
		"N8N_LISTEN_ADDRESS":  "0.0.0.0",
		"N8N_PUSH_BACKEND":    "sse",
		"N8N_PROXY_HOPS":      "1",
	}

	u := strings.TrimSpace(publicURL)
	if u == "" {
		return out
	}
	parsed, err := url.Parse(u)
	if err != nil {
		return out
	}

	scheme := strings.TrimSpace(parsed.Scheme)
	host := strings.TrimSpace(parsed.Hostname())
	if scheme == "" {
		scheme = "https"
	}
	if host == "" {
		return out
	}

	base := strings.TrimRight(scheme+"://"+host, "/") + "/"
	out["N8N_PROTOCOL"] = scheme
	out["N8N_HOST"] = host
	out["N8N_EDITOR_BASE_URL"] = base
	out["WEBHOOK_URL"] = base
	return out
}

func resolveN8NDatabaseEnv(existing map[string]string) (map[string]string, error) {
	dbURL := strings.TrimSpace(n8nDBURL)
	if dbURL == "" && !n8nRedeploy && !hasPostgresConfig(existing) {
		useDB, err := promptYesNo("Attach external Postgres (recommended, e.g. Supabase)?", false)
		if err != nil {
			return nil, err
		}
		if useDB {
			s, err := promptLine("Enter Postgres URL")
			if err != nil {
				return nil, err
			}
			dbURL = strings.TrimSpace(s)
		}
	}
	if dbURL == "" {
		if hasPostgresConfig(existing) {
			return extractPostgresEnv(existing), nil
		}
		return map[string]string{}, nil
	}

	env, err := postgresEnvFromURL(dbURL, strings.TrimSpace(n8nDBSchema), n8nDBSSLReject)
	if err != nil {
		return nil, err
	}
	return env, nil
}

func resolveN8NEncryptionKey(existing map[string]string) (string, error) {
	if v := strings.TrimSpace(n8nEncryptKey); v != "" {
		return v, nil
	}
	if v := strings.TrimSpace(existing["N8N_ENCRYPTION_KEY"]); v != "" {
		return v, nil
	}
	if hasPostgresConfig(existing) {
		// Existing setup without key: keep behavior unchanged.
		return "", nil
	}
	if n8nRedeploy {
		return "", nil
	}

	want, err := promptYesNo("Set stable N8N_ENCRYPTION_KEY now? (recommended)", true)
	if err != nil {
		return "", err
	}
	if !want {
		return "", nil
	}

	v, err := promptLine("Enter N8N_ENCRYPTION_KEY (leave empty to auto-generate)")
	if err != nil {
		return "", err
	}
	v = strings.TrimSpace(v)
	if v == "" {
		v = randomSecretHex(32)
		fmt.Println("Generated N8N_ENCRYPTION_KEY for this deployment.")
	}
	return v, nil
}

func postgresEnvFromURL(rawURL, schema string, sslRejectUnauthorized bool) (map[string]string, error) {
	u, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return nil, fmt.Errorf("invalid --db-url: %w", err)
	}
	scheme := strings.ToLower(strings.TrimSpace(u.Scheme))
	if scheme != "postgres" && scheme != "postgresql" {
		return nil, fmt.Errorf("invalid --db-url scheme %q: expected postgres:// or postgresql://", u.Scheme)
	}

	host := strings.TrimSpace(u.Hostname())
	if host == "" {
		return nil, fmt.Errorf("invalid --db-url: missing host")
	}
	port := strings.TrimSpace(u.Port())
	if port == "" {
		port = "5432"
	}

	dbName := strings.TrimPrefix(strings.TrimSpace(u.Path), "/")
	if dbName == "" {
		return nil, fmt.Errorf("invalid --db-url: missing database name in path")
	}

	user := ""
	pass := ""
	if u.User != nil {
		user = strings.TrimSpace(u.User.Username())
		pass, _ = u.User.Password()
		pass = strings.TrimSpace(pass)
	}
	if user == "" {
		return nil, fmt.Errorf("invalid --db-url: missing username")
	}
	if pass == "" {
		return nil, fmt.Errorf("invalid --db-url: missing password")
	}

	sslMode := strings.ToLower(strings.TrimSpace(u.Query().Get("sslmode")))
	sslEnabled := sslMode != "disable"

	out := map[string]string{
		"DB_TYPE":                   "postgresdb",
		"DB_POSTGRESDB_HOST":        host,
		"DB_POSTGRESDB_PORT":        port,
		"DB_POSTGRESDB_DATABASE":    dbName,
		"DB_POSTGRESDB_USER":        user,
		"DB_POSTGRESDB_PASSWORD":    pass,
		"DB_POSTGRESDB_SSL_ENABLED": strconv.FormatBool(sslEnabled),
	}
	if strings.TrimSpace(schema) != "" {
		out["DB_POSTGRESDB_SCHEMA"] = strings.TrimSpace(schema)
	}
	if sslEnabled {
		out["DB_POSTGRESDB_SSL_REJECT_UNAUTHORIZED"] = strconv.FormatBool(sslRejectUnauthorized)
	}
	return out, nil
}

func promptYesNo(question string, defaultYes bool) (bool, error) {
	in := bufio.NewReader(os.Stdin)
	def := "y/N"
	if defaultYes {
		def = "Y/n"
	}
	for {
		fmt.Printf("%s [%s]: ", question, def)
		s, err := in.ReadString('\n')
		if err != nil {
			return false, err
		}
		s = strings.TrimSpace(strings.ToLower(s))
		if s == "" {
			return defaultYes, nil
		}
		switch s {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Println("Please answer y or n.")
		}
	}
}

func promptLine(question string) (string, error) {
	in := bufio.NewReader(os.Stdin)
	fmt.Printf("%s: ", question)
	s, err := in.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(s), nil
}

func hasPostgresConfig(env map[string]string) bool {
	return strings.EqualFold(strings.TrimSpace(env["DB_TYPE"]), "postgresdb")
}

func extractPostgresEnv(env map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range env {
		if k == "DB_TYPE" || strings.HasPrefix(k, "DB_POSTGRESDB_") {
			out[k] = v
		}
	}
	return out
}

func randomSecretHex(bytesN int) string {
	if bytesN <= 0 {
		bytesN = 32
	}
	b := make([]byte, bytesN)
	if _, err := rand.Read(b); err == nil {
		return hex.EncodeToString(b)
	}
	return strings.ToLower(time.Now().UTC().Format("20060102150405"))
}

func copyStringMap(in map[string]string) map[string]string {
	out := map[string]string{}
	for k, v := range in {
		out[k] = v
	}
	return out
}

func mergeEnv(base, overlay map[string]string) map[string]string {
	out := copyStringMap(base)
	for k, v := range overlay {
		out[k] = v
	}
	return out
}

func sameStringMap(a, b map[string]string) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if b[k] != v {
			return false
		}
	}
	return true
}

func isServiceNotFound(err error) bool {
	ae, ok := err.(*apperr.Error)
	if !ok {
		return false
	}
	if ae.Code != gcprun.ErrRunGetService.Code {
		return false
	}
	return strings.HasPrefix(strings.TrimSpace(ae.Meta["http_status"]), "404")
}

func createProjectWithAutoSuffix(ctx context.Context, accessToken, projectID, name string) (string, error) {
	base := strings.TrimSpace(projectID)
	candidate := base

	for attempt := 0; attempt < 6; attempt++ {
		fmt.Printf("Creating project %q...\n", candidate)
		err := gcpcrm.CreateProject(ctx, accessToken, candidate, name)
		if err == nil {
			fmt.Printf("✓ Project creation request accepted: %s\n", candidate)
			return candidate, nil
		}
		if !isProjectAlreadyExistsError(err) {
			return "", err
		}

		next := projectIDWithHashSuffix(base, randomHashSuffix())
		fmt.Printf("Project id %q is already taken. Trying %q...\n", candidate, next)
		candidate = next
	}

	return "", fmt.Errorf("failed to allocate a unique project id based on %q after multiple attempts", base)
}

func isProjectAlreadyExistsError(err error) bool {
	ae, ok := err.(*apperr.Error)
	if !ok {
		return false
	}
	if ae.Code != gcpcrm.ErrProjectCreate.Code {
		return false
	}

	status := strings.TrimSpace(ae.Meta["http_status"])
	body := strings.ToUpper(strings.TrimSpace(ae.Meta["raw_body"]))
	if strings.HasPrefix(status, "409") {
		return true
	}
	return strings.Contains(body, "ALREADY_EXISTS") || strings.Contains(body, "REQUESTED ENTITY ALREADY EXISTS")
}

func projectIDWithHashSuffix(base, suffix string) string {
	suffix = strings.TrimSpace(strings.ToLower(suffix))
	if suffix == "" {
		suffix = "x1"
	}

	base = strings.TrimSpace(strings.ToLower(base))
	base = strings.Trim(base, "-")
	if base == "" {
		base = "proj"
	}

	maxBase := 30 - 1 - len(suffix)
	if maxBase < 1 {
		maxBase = 1
	}
	if len(base) > maxBase {
		base = base[:maxBase]
	}
	base = strings.TrimRight(base, "-")
	if base == "" {
		base = "p"
	}

	out := base + "-" + suffix
	if len(out) < 6 {
		out = out + "000000"
		out = out[:6]
	}
	return out
}

func randomHashSuffix() string {
	var b [3]byte
	if _, err := rand.Read(b[:]); err == nil {
		return hex.EncodeToString(b[:]) // 6 hex chars
	}
	return strings.ToLower(time.Now().UTC().Format("150405"))
}
