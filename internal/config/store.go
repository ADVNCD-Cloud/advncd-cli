package config

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	StoreReadFailed   = apperr.E("B-CONFIG-001", "Failed to read config")
	StoreWriteFailed  = apperr.E("B-CONFIG-002", "Failed to write config")
	StoreDeleteFailed = apperr.E("B-CONFIG-003", "Failed to delete config")
)

type Store struct {
	Path string
}

func DefaultStore() (*Store, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, apperr.New(StoreWriteFailed).WithCause(err).
			WithFix("Unable to resolve user config dir.")
	}
	base := filepath.Join(dir, "advncd")
	return &Store{Path: filepath.Join(base, "config.json")}, nil
}

func (s *Store) EnsureDir() error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return apperr.New(StoreWriteFailed).WithCause(err).
			WithFix("Check filesystem permissions.")
	}
	return nil
}

func (s *Store) Save(c Config) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return apperr.New(StoreWriteFailed).WithCause(err)
	}

	if err := os.WriteFile(s.Path, b, 0o600); err != nil {
		return apperr.New(StoreWriteFailed).WithCause(err).
			WithFix("Check filesystem permissions.")
	}
	return nil
}

func (s *Store) Load() (*Config, error) {
	b, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, apperr.New(StoreReadFailed).WithCause(err)
	}

	var c Config
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, apperr.New(StoreReadFailed).WithCause(err).
			WithFix("Config file is corrupted; re-run: advncd init")
	}
	return &c, nil
}

func (s *Store) Delete() error {
	if err := os.Remove(s.Path); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return apperr.New(StoreDeleteFailed).WithCause(err)
	}
	return nil
}