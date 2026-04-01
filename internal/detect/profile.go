package detect

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/projectslug"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/publishplan"
)

type Result struct {
	Path            string
	ServiceProposal string
	Profile         contracts.DeployProfile
}

func Project(path string) (Result, error) {
	root := strings.TrimSpace(path)
	if root == "" {
		return Result{}, fmt.Errorf("path is required")
	}

	st, err := os.Stat(root)
	if err != nil {
		return Result{}, err
	}
	if !st.IsDir() {
		return Result{}, fmt.Errorf("path is not a directory: %s", root)
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return Result{}, err
	}

	scanned, err := publishplan.Scan(absRoot)
	if err != nil {
		return Result{}, err
	}

	profile := profileFromStack(scanned.Stack)
	profile.Port = scanned.Port
	signalSet := detectSignals(absRoot)

	if signalSet.has("Dockerfile") {
		profile.Runtime = "dockerfile"
		profile.BuildStrategy = "dockerfile"
		profile.StartStrategy = "docker_entrypoint"
		if port, ok := inferDockerExposePort(filepath.Join(absRoot, "Dockerfile")); ok {
			profile.Port = port
		}
	}
	if looksStaticBundle(absRoot) {
		profile.Runtime = "static"
		profile.AppKind = "static"
		profile.BuildStrategy = "static_bundle"
		profile.StartStrategy = "inferred"
	}

	if profile.Port <= 0 {
		profile.Port = 8080
		profile.Warnings = append(profile.Warnings, "using default port 8080")
	}

	if profile.BuildStrategy == "" {
		profile.BuildStrategy = "manual"
	}
	if profile.StartStrategy == "" {
		profile.StartStrategy = "inferred"
	}
	if profile.AppKind == "" {
		profile.AppKind = "unknown"
	}

	if profile.Runtime == "unknown" || profile.BuildStrategy == "manual" {
		profile.Confidence = "low"
		profile.Warnings = append(profile.Warnings, "runtime detection is uncertain")
	} else if len(signalSet.items) > 0 {
		profile.Confidence = "high"
	} else {
		profile.Confidence = "medium"
	}

	profile.Signals = signalSet.list()

	service := projectslug.FromPathBase(absRoot)
	if strings.TrimSpace(service) == "" {
		service = "app"
	}

	return Result{
		Path:            absRoot,
		ServiceProposal: service,
		Profile:         profile,
	}, nil
}

func profileFromStack(stack string) contracts.DeployProfile {
	profile := contracts.DeployProfile{
		Runtime:       "unknown",
		Framework:     "",
		AppKind:       "unknown",
		BuildStrategy: "buildpacks",
		StartStrategy: "buildpack_default",
		Port:          8080,
		Warnings:      []string{},
		Signals:       []string{},
	}

	switch stack {
	case publishplan.StackNextJS, publishplan.StackNuxt, publishplan.StackSvelteKit, publishplan.StackRemix, publishplan.StackAstro, publishplan.StackAngularSSR, publishplan.StackViteSPA:
		profile.Runtime = "node"
		profile.Framework = stack
		profile.AppKind = "web"
	case publishplan.StackExpress, publishplan.StackNestJS, publishplan.StackFastify, publishplan.StackKoa, publishplan.StackHono:
		profile.Runtime = "node"
		profile.Framework = stack
		profile.AppKind = "api"
	case publishplan.StackNode:
		profile.Runtime = "node"
		profile.AppKind = "web"
	case publishplan.StackFastAPI, publishplan.StackFlask, publishplan.StackDjango:
		profile.Runtime = "python"
		profile.Framework = stack
		profile.AppKind = "api"
	case publishplan.StackPython:
		profile.Runtime = "python"
		profile.AppKind = "api"
	case publishplan.StackLaravel, publishplan.StackSymfony:
		profile.Runtime = "php"
		profile.Framework = stack
		profile.AppKind = "web"
	case publishplan.StackPHP:
		profile.Runtime = "php"
		profile.AppKind = "web"
	case publishplan.StackGin, publishplan.StackEcho, publishplan.StackFiber:
		profile.Runtime = "go"
		profile.Framework = stack
		profile.AppKind = "api"
	case publishplan.StackGo:
		profile.Runtime = "go"
		profile.AppKind = "api"
	case publishplan.StackSpringBoot, publishplan.StackQuarkus, publishplan.StackMicronaut, publishplan.StackKtor:
		profile.Runtime = "java"
		profile.Framework = stack
		profile.AppKind = "api"
	case publishplan.StackJava:
		profile.Runtime = "java"
		profile.AppKind = "api"
	case publishplan.StackDotNet:
		profile.Runtime = "dotnet"
		profile.AppKind = "api"
	case publishplan.StackRails, publishplan.StackSinatra:
		profile.Runtime = "ruby"
		profile.Framework = stack
		profile.AppKind = "web"
	case publishplan.StackRuby:
		profile.Runtime = "ruby"
		profile.AppKind = "web"
	case publishplan.StackRust:
		profile.Runtime = "rust"
		profile.AppKind = "api"
	default:
		profile.Runtime = "unknown"
	}
	return profile
}

type signals struct {
	items map[string]struct{}
}

func newSignals() *signals {
	return &signals{items: map[string]struct{}{}}
}

func (s *signals) add(v string) {
	s.items[v] = struct{}{}
}

func (s *signals) has(v string) bool {
	_, ok := s.items[v]
	return ok
}

func (s *signals) list() []string {
	out := make([]string, 0, len(s.items))
	for k := range s.items {
		out = append(out, k)
	}
	// keep stable output
	for i := 0; i < len(out); i++ {
		for j := i + 1; j < len(out); j++ {
			if out[j] < out[i] {
				out[i], out[j] = out[j], out[i]
			}
		}
	}
	return out
}

func detectSignals(root string) *signals {
	s := newSignals()
	candidates := []string{
		"package.json",
		"requirements.txt",
		"pyproject.toml",
		"Pipfile",
		"composer.json",
		"go.mod",
		"pom.xml",
		"build.gradle",
		"build.gradle.kts",
		"Dockerfile",
		"Gemfile",
		"Cargo.toml",
	}
	for _, name := range candidates {
		if fileExists(filepath.Join(root, name)) {
			s.add(name)
		}
	}
	if looksStaticBundle(root) {
		s.add("static_output")
	}
	return s
}

func looksStaticBundle(root string) bool {
	dirs := []string{"dist", "build", "public"}
	for _, name := range dirs {
		p := filepath.Join(root, name)
		st, err := os.Stat(p)
		if err == nil && st.IsDir() {
			if fileExists(filepath.Join(p, "index.html")) {
				return true
			}
		}
	}
	return false
}

func inferDockerExposePort(dockerfilePath string) (int, bool) {
	b, err := os.ReadFile(dockerfilePath)
	if err != nil {
		return 0, false
	}
	lines := strings.Split(string(b), "\n")
	for _, line := range lines {
		l := strings.TrimSpace(strings.ToUpper(line))
		if !strings.HasPrefix(l, "EXPOSE ") {
			continue
		}
		raw := strings.TrimSpace(line[len("EXPOSE "):])
		fields := strings.Fields(raw)
		if len(fields) == 0 {
			continue
		}
		p := strings.Split(fields[0], "/")[0]
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err == nil && n > 0 {
			return n, true
		}
	}
	return 0, false
}

func fileExists(path string) bool {
	st, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !st.IsDir()
}
