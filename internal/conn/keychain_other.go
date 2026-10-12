//go:build !darwin

package conn

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// FileSecretStore implements SecretStore with dir/credentials.json (0600,
// atomic writes) for non-macOS platforms.
type FileSecretStore struct {
	mu   sync.Mutex
	path string
	m    map[string]string
}

// NewFileSecretStore loads (or initializes) the credential file in dir.
func NewFileSecretStore(dir string) (*FileSecretStore, error) {
	s := &FileSecretStore{
		path: filepath.Join(dir, "credentials.json"),
		m:    map[string]string{},
	}
	raw, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(raw, &s.m); err != nil {
		return nil, fmt.Errorf("conn: parse %s: %w", s.path, err)
	}
	return s, nil
}

// Set stores the secret and persists the file atomically.
func (s *FileSecretStore) Set(account, secret string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[account] = secret
	return s.save()
}

// Get returns the secret for account, "" when absent.
func (s *FileSecretStore) Get(account string) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.m[account]
}

// Delete removes the secret (missing is fine) and persists.
func (s *FileSecretStore) Delete(account string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, account)
	return s.save()
}

func (s *FileSecretStore) save() error {
	raw, err := json.MarshalIndent(s.m, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.path), ".credentials-*.json")
	if err != nil {
		return err
	}
	name := tmp.Name()
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(name)
		return err
	}
	if err := tmp.Close(); err != nil {
		os.Remove(name)
		return err
	}
	return os.Rename(name, s.path)
}
