//go:build !darwin

package conn

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileSecretStore(t *testing.T) {
	dir := t.TempDir()
	fss, err := NewFileSecretStore(dir)
	if err != nil {
		t.Fatalf("NewFileSecretStore: %v", err)
	}
	if got := fss.Get("a"); got != "" {
		t.Fatalf("Get on empty store = %q", got)
	}
	if err := fss.Set("a", "secret"); err != nil {
		t.Fatalf("Set: %v", err)
	}
	if got := fss.Get("a"); got != "secret" {
		t.Fatalf("Get = %q", got)
	}

	fi, err := os.Stat(filepath.Join(dir, "credentials.json"))
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("credentials.json mode = %v, want 0600", fi.Mode().Perm())
	}

	// Reload from disk keeps the value.
	fss2, err := NewFileSecretStore(dir)
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if got := fss2.Get("a"); got != "secret" {
		t.Fatalf("after reload Get = %q", got)
	}
	if err := fss2.Delete("a"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if got := fss2.Get("a"); got != "" {
		t.Fatalf("after Delete Get = %q", got)
	}
}

func TestFileSecretStoreWithStore(t *testing.T) {
	dir := t.TempDir()
	fss, err := NewFileSecretStore(dir)
	if err != nil {
		t.Fatalf("NewFileSecretStore: %v", err)
	}
	s, err := NewStore(dir, fss)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	id, err := s.Add(Connection{Name: "b", AuthType: AuthBasic, Username: "u"}, Credentials{Password: "pw"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	// Fresh instances see the same credentials.
	fss2, _ := NewFileSecretStore(dir)
	s2, _ := NewStore(dir, fss2)
	if c := s2.Credentials(id); c.Password != "pw" {
		t.Fatalf("Credentials after reload = %+v", c)
	}
}
