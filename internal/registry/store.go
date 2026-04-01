package registry

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/contracts"
)

type Store struct {
	Path string
}

func DefaultStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, err
	}
	return &Store{Path: filepath.Join(dir, "advncd", "registry.json")}, nil
}

func (s *Store) EnsureDir() error {
	return os.MkdirAll(filepath.Dir(s.Path), 0o700)
}

func (s *Store) Load() (contracts.Registry, error) {
	raw, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return contracts.NewRegistry(), nil
		}
		return contracts.Registry{}, err
	}

	var r contracts.Registry
	if err := json.Unmarshal(raw, &r); err != nil {
		return contracts.Registry{}, err
	}
	if r.Version == 0 {
		r.Version = contracts.RegistryVersion
	}
	if r.Deployments == nil {
		r.Deployments = []contracts.DeploymentRecord{}
	}
	return r, nil
}

func (s *Store) Save(r contracts.Registry) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}
	if r.Version == 0 {
		r.Version = contracts.RegistryVersion
	}
	if r.Deployments == nil {
		r.Deployments = []contracts.DeploymentRecord{}
	}
	raw, err := json.MarshalIndent(r, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.Path, raw, 0o600)
}

func (s *Store) AddDeployment(rec contracts.DeploymentRecord) error {
	reg, err := s.Load()
	if err != nil {
		return err
	}
	if strings.TrimSpace(rec.DeploymentID) == "" {
		rec.DeploymentID = newDeploymentID()
	}
	now := time.Now().UTC()
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	reg.Deployments = append(reg.Deployments, rec)
	return s.Save(reg)
}

func (s *Store) UpdateDeployment(rec contracts.DeploymentRecord) error {
	if strings.TrimSpace(rec.DeploymentID) == "" {
		return fmt.Errorf("deployment_id is required")
	}
	reg, err := s.Load()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	for i := range reg.Deployments {
		if reg.Deployments[i].DeploymentID == rec.DeploymentID {
			if rec.CreatedAt.IsZero() {
				rec.CreatedAt = reg.Deployments[i].CreatedAt
			}
			if rec.CreatedAt.IsZero() {
				rec.CreatedAt = now
			}
			rec.UpdatedAt = now
			reg.Deployments[i] = rec
			return s.Save(reg)
		}
	}
	return fmt.Errorf("deployment_id not found: %s", rec.DeploymentID)
}

func (s *Store) RemoveDeployment(deploymentID string) error {
	reg, err := s.Load()
	if err != nil {
		return err
	}
	out := make([]contracts.DeploymentRecord, 0, len(reg.Deployments))
	for _, rec := range reg.Deployments {
		if rec.DeploymentID == deploymentID {
			continue
		}
		out = append(out, rec)
	}
	reg.Deployments = out
	return s.Save(reg)
}

func (s *Store) ResolveDeploymentByID(deploymentID string) (*contracts.DeploymentRecord, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}
	for _, rec := range reg.Deployments {
		if rec.DeploymentID == deploymentID {
			cp := rec
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *Store) ResolveDeploymentByServiceIdentity(identity contracts.ServiceIdentity) (*contracts.DeploymentRecord, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}
	key := identity.Key()
	for _, rec := range reg.Deployments {
		if rec.Identity().Key() == key {
			cp := rec
			return &cp, nil
		}
	}
	return nil, nil
}

func (s *Store) ListRecentDeployments(limit int) ([]contracts.DeploymentRecord, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}
	out := append([]contracts.DeploymentRecord(nil), reg.Deployments...)
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (s *Store) ListDeploymentsByProject(projectID string) ([]contracts.DeploymentRecord, error) {
	reg, err := s.Load()
	if err != nil {
		return nil, err
	}
	out := make([]contracts.DeploymentRecord, 0, len(reg.Deployments))
	for _, rec := range reg.Deployments {
		if rec.ProjectID == projectID {
			out = append(out, rec)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return out[i].UpdatedAt.After(out[j].UpdatedAt)
	})
	return out, nil
}

func (s *Store) UpsertDeploymentByServiceIdentity(rec contracts.DeploymentRecord) error {
	reg, err := s.Load()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	key := rec.Identity().Key()
	for i := range reg.Deployments {
		if reg.Deployments[i].Identity().Key() != key {
			continue
		}
		if strings.TrimSpace(rec.DeploymentID) == "" {
			rec.DeploymentID = reg.Deployments[i].DeploymentID
		}
		if rec.CreatedAt.IsZero() {
			rec.CreatedAt = reg.Deployments[i].CreatedAt
		}
		rec.UpdatedAt = now
		reg.Deployments[i] = rec
		return s.Save(reg)
	}

	if strings.TrimSpace(rec.DeploymentID) == "" {
		rec.DeploymentID = newDeploymentID()
	}
	if rec.CreatedAt.IsZero() {
		rec.CreatedAt = now
	}
	rec.UpdatedAt = now
	reg.Deployments = append(reg.Deployments, rec)
	return s.Save(reg)
}

func newDeploymentID() string {
	return fmt.Sprintf("dep_%d", time.Now().UTC().UnixNano())
}
