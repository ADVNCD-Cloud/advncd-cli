package creds

import (
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/ADVNCD-Cloud/advncd-cli/internal/apperr"
)

var (
	StoreReadFailed  = apperr.E("A-CREDS-001", "Failed to read credentials")
	StoreWriteFailed = apperr.E("A-CREDS-002", "Failed to write credentials")
	StoreDeleteFailed = apperr.E("A-CREDS-003", "Failed to delete credentials")
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
	return &Store{Path: filepath.Join(base, "credentials.json")}, nil
}

func (s *Store) EnsureDir() error {
	if err := os.MkdirAll(filepath.Dir(s.Path), 0o700); err != nil {
		return apperr.New(StoreWriteFailed).WithCause(err).
			WithFix("Check filesystem permissions.")
	}
	return nil
}

func (s *Store) Save(c Credentials) error {
	if err := s.EnsureDir(); err != nil {
		return err
	}

	b, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return apperr.New(StoreWriteFailed).WithCause(err)
	}

	// 0600 — only user can read/write
	if err := os.WriteFile(s.Path, b, 0o600); err != nil {
		return apperr.New(StoreWriteFailed).WithCause(err).
			WithFix("Check filesystem permissions.")
	}
	return nil
}

func (s *Store) Load() (*Credentials, error) {
	b, err := os.ReadFile(s.Path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, apperr.New(StoreReadFailed).WithCause(err).
			WithFix("Check filesystem permissions.")
	}

	var c Credentials
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, apperr.New(StoreReadFailed).WithCause(err).
			WithFix("Credentials file is corrupted; try 'advncd logout' and login again.")
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