package dashboard

import (
	"sort"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/gcprun"
	"github.com/ADVNCD-Cloud/advncd-cli/internal/registry"
)

type serviceState struct {
	Name               string
	SourceType         string
	Status             string
	URL                string
	UpdatedAt          time.Time
	LastRevision       string
	DeploymentID       string
	SourcePathOptional string
	PresetIDOptional   string
	ProjectID          string
	Region             string
}

func loadRegistryServiceState(projectID, region string) (map[string]serviceState, error) {
	store, err := registry.DefaultStore()
	if err != nil {
		return nil, err
	}

	records, err := store.ListDeploymentsByProject(projectID)
	if err != nil {
		return nil, err
	}

	out := make(map[string]serviceState, len(records))
	for _, rec := range records {
		if strings.TrimSpace(rec.Region) != strings.TrimSpace(region) {
			continue
		}
		name := strings.TrimSpace(rec.ServiceName)
		if name == "" {
			continue
		}

		if _, exists := out[name]; exists {
			// ListDeploymentsByProject is already sorted by updated desc.
			continue
		}

		out[name] = serviceState{
			Name:               name,
			SourceType:         string(rec.SourceType),
			Status:             normalizeStatus(rec.Status),
			URL:                strings.TrimSpace(rec.URL),
			UpdatedAt:          rec.UpdatedAt,
			LastRevision:       strings.TrimSpace(rec.LastRevision),
			DeploymentID:       strings.TrimSpace(rec.DeploymentID),
			SourcePathOptional: strings.TrimSpace(rec.SourcePathOptional),
			PresetIDOptional:   strings.TrimSpace(rec.PresetIDOptional),
			ProjectID:          strings.TrimSpace(rec.ProjectID),
			Region:             strings.TrimSpace(rec.Region),
		}
	}

	return out, nil
}

func reconcileLiveServices(in map[string]serviceState, projectID, region string, live []gcprun.Service) map[string]serviceState {
	out := make(map[string]serviceState, len(in)+len(live))
	for k, v := range in {
		out[k] = v
	}

	for _, svc := range live {
		name := strings.TrimSpace(svc.Name)
		if name == "" {
			continue
		}

		row, ok := out[name]
		if !ok {
			row = serviceState{
				Name:       name,
				SourceType: "",
				ProjectID:  strings.TrimSpace(projectID),
				Region:     strings.TrimSpace(region),
			}
		}

		row.Status = normalizeStatus(svc.Status)
		if strings.TrimSpace(svc.URL) != "" {
			row.URL = strings.TrimSpace(svc.URL)
		}
		if !svc.UpdatedAt.IsZero() {
			row.UpdatedAt = svc.UpdatedAt
		}
		if strings.TrimSpace(row.ProjectID) == "" {
			row.ProjectID = strings.TrimSpace(projectID)
		}
		if strings.TrimSpace(row.Region) == "" {
			row.Region = strings.TrimSpace(region)
		}

		out[name] = row
	}

	return out
}

func stateMapToSortedList(in map[string]serviceState) []serviceState {
	out := make([]serviceState, 0, len(in))
	for _, v := range in {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].UpdatedAt.Equal(out[j].UpdatedAt) {
			return out[i].Name < out[j].Name
		}
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out
}

func normalizeStatus(v string) string {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "ready":
		return "ready"
	case "error":
		return "error"
	default:
		return "unknown"
	}
}

func loadRecentContextHints(limit int) ([]string, []string, error) {
	store, err := registry.DefaultStore()
	if err != nil {
		return nil, nil, err
	}
	records, err := store.ListRecentDeployments(limit)
	if err != nil {
		return nil, nil, err
	}

	projects := make([]string, 0, len(records))
	regions := make([]string, 0, len(records))
	seenProjects := map[string]bool{}
	seenRegions := map[string]bool{}

	for _, rec := range records {
		projectID := strings.TrimSpace(rec.ProjectID)
		region := strings.TrimSpace(rec.Region)

		if projectID != "" && !seenProjects[projectID] {
			seenProjects[projectID] = true
			projects = append(projects, projectID)
		}
		if region != "" && !seenRegions[region] {
			seenRegions[region] = true
			regions = append(regions, region)
		}
	}

	return projects, regions, nil
}

func upsertServiceStateToRegistry(svc serviceState) error {
	store, err := registry.DefaultStore()
	if err != nil {
		return err
	}

	rec := contracts.DeploymentRecord{
		DeploymentID:       svc.DeploymentID,
		SourceType:         contracts.SourceTypeCode,
		SourcePathOptional: svc.SourcePathOptional,
		PresetIDOptional:   svc.PresetIDOptional,
		ServiceName:        svc.Name,
		ProjectID:          svc.ProjectID,
		Region:             svc.Region,
		URL:                svc.URL,
		Status:             normalizeStatus(svc.Status),
		LastRevision:       svc.LastRevision,
	}

	if strings.TrimSpace(svc.SourceType) == string(contracts.SourceTypePreset) {
		rec.SourceType = contracts.SourceTypePreset
	}

	return store.UpsertDeploymentByServiceIdentity(rec)
}

func removeServiceStateFromRegistry(serviceName, projectID, region string) error {
	store, err := registry.DefaultStore()
	if err != nil {
		return err
	}
	rec, err := store.ResolveDeploymentByServiceIdentity(contracts.ServiceIdentity{
		ServiceName: serviceName,
		ProjectID:   projectID,
		Region:      region,
	})
	if err != nil {
		return err
	}
	if rec == nil || strings.TrimSpace(rec.DeploymentID) == "" {
		return nil
	}
	return store.RemoveDeployment(rec.DeploymentID)
}
