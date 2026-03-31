package contracts

import (
	"strings"
	"time"
)

// Context is the shared CLI/dashboard execution context contract.
type Context struct {
	AccountEmail    string `json:"account_email"`
	ActiveProjectID string `json:"active_project_id"`
	ActiveRegion    string `json:"active_region"`
	AuthState       string `json:"auth_state"`
}

type SourceType string

const (
	SourceTypeCode   SourceType = "code"
	SourceTypePreset SourceType = "preset"
)

func (t SourceType) Valid() bool {
	return t == SourceTypeCode || t == SourceTypePreset
}

// Source is the shared source contract with locked code/preset distinctions.
type Source struct {
	SourceType         SourceType `json:"source_type"`
	SourcePathOptional string     `json:"source_path_optional,omitempty"`
	PresetIDOptional   string     `json:"preset_id_optional,omitempty"`
}

// DeployProfile is the shared detect/deploy profile contract.
type DeployProfile struct {
	Runtime       string   `json:"runtime"`
	Framework     string   `json:"framework"`
	AppKind       string   `json:"app_kind"`
	BuildStrategy string   `json:"build_strategy"`
	StartStrategy string   `json:"start_strategy"`
	Port          int      `json:"port"`
	Confidence    string   `json:"confidence"`
	Warnings      []string `json:"warnings"`
	Signals       []string `json:"signals"`
}

type ServiceIdentity struct {
	ServiceName string `json:"service_name"`
	ProjectID   string `json:"project_id"`
	Region      string `json:"region"`
}

func (i ServiceIdentity) Complete() bool {
	return strings.TrimSpace(i.ServiceName) != "" &&
		strings.TrimSpace(i.ProjectID) != "" &&
		strings.TrimSpace(i.Region) != ""
}

func (i ServiceIdentity) Key() string {
	return strings.TrimSpace(i.ServiceName) + ":" + strings.TrimSpace(i.ProjectID) + ":" + strings.TrimSpace(i.Region)
}

// DeploymentRecord is the registry-facing deployment schema for v1.
type DeploymentRecord struct {
	DeploymentID       string     `json:"deployment_id"`
	SourceType         SourceType `json:"source_type"`
	SourcePathOptional string     `json:"source_path_optional,omitempty"`
	PresetIDOptional   string     `json:"preset_id_optional,omitempty"`
	ServiceName        string     `json:"service_name"`
	ProjectID          string     `json:"project_id"`
	Region             string     `json:"region"`
	URL                string     `json:"url"`
	Status             string     `json:"status"`
	LastRevision       string     `json:"last_revision"`
	CreatedAt          time.Time  `json:"created_at"`
	UpdatedAt          time.Time  `json:"updated_at"`
}

func (r DeploymentRecord) Identity() ServiceIdentity {
	return ServiceIdentity{
		ServiceName: r.ServiceName,
		ProjectID:   r.ProjectID,
		Region:      r.Region,
	}
}

// PresetDefinition is the shared ready-made service definition contract.
type PresetDefinition struct {
	PresetID           string   `json:"preset_id"`
	DisplayName        string   `json:"display_name"`
	Category           string   `json:"category"`
	Summary            string   `json:"summary"`
	LaunchMode         string   `json:"launch_mode"`
	DefaultNamePattern string   `json:"default_name_pattern"`
	RequiredInputs     []string `json:"required_inputs"`
	OptionalInputs     []string `json:"optional_inputs"`
}

const RegistryVersion = 1

// Registry freezes the local registry shape for deployment records.
type Registry struct {
	Version     int                `json:"version"`
	Deployments []DeploymentRecord `json:"deployments"`
}

func NewRegistry() Registry {
	return Registry{
		Version:     RegistryVersion,
		Deployments: []DeploymentRecord{},
	}
}

const (
	CommandDetect    = "detect"
	CommandDeploy    = "deploy"
	CommandLaunch    = "launch"
	CommandServices  = "services"
	CommandDashboard = "dashboard"
)

func StableCLICommands() []string {
	return []string{
		CommandDetect,
		CommandDeploy,
		CommandLaunch,
		CommandServices,
		CommandDashboard,
	}
}
