package conn

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"seismic/internal/es"
)

// fakeSecretStore is the injectable SecretStore used by all tests.
type fakeSecretStore struct {
	m        map[string]string
	setCalls int
}

func newFake() *fakeSecretStore { return &fakeSecretStore{m: map[string]string{}} }

func (f *fakeSecretStore) Set(account, secret string) error {
	f.setCalls++
	f.m[account] = secret
	return nil
}
func (f *fakeSecretStore) Get(account string) string { return f.m[account] }
func (f *fakeSecretStore) Delete(account string) error {
	delete(f.m, account)
	return nil
}

func newTestStore(t *testing.T) (*Store, *fakeSecretStore, string) {
	t.Helper()
	dir := t.TempDir()
	fake := newFake()
	s, err := NewStore(dir, fake)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return s, fake, dir
}

func TestStoreCRUD(t *testing.T) {
	s, fake, dir := newTestStore(t)

	id, err := s.Add(Connection{Name: "local", URI: "http://localhost:9200", AuthType: AuthBasic, Username: "elastic"}, Credentials{Password: "pw"})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}
	if id == "" || len(id) != 36 {
		t.Fatalf("Add returned bad UUID %q", id)
	}
	if len(s.List()) != 1 {
		t.Fatalf("List = %d, want 1", len(s.List()))
	}
	got, ok := s.Get(id)
	if !ok || got.Name != "local" || got.Username != "elastic" {
		t.Fatalf("Get = %+v, %v", got, ok)
	}

	// Secret roundtrip via the fake store: only Password survives for basic.
	if _, ok := fake.m[id]; !ok {
		t.Fatal("Add did not store a secret")
	}
	creds := s.Credentials(id)
	if creds.Password != "pw" || creds.APIKey != "" || creds.AccessKeyID != "" {
		t.Fatalf("Credentials = %+v", creds)
	}

	// Update name; keep credentials when none are supplied.
	got.Name = "renamed"
	if err := s.Update(got, Credentials{}); err != nil {
		t.Fatalf("Update: %v", err)
	}
	if c, _ := s.Get(id); c.Name != "renamed" {
		t.Fatalf("after Update, name = %q", c.Name)
	}
	if s.Credentials(id).Password != "pw" {
		t.Fatal("Update wiped credentials")
	}

	// Second connection; UUIDs differ.
	id2, err := s.Add(Connection{Name: "os", URI: "http://os:9200", AuthType: AuthNone}, Credentials{})
	if err != nil {
		t.Fatalf("Add 2: %v", err)
	}
	if id2 == id {
		t.Fatal("duplicate UUID")
	}

	// Delete removes both metadata and secret.
	if err := s.Delete(id); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, ok := s.Get(id); ok {
		t.Fatal("connection still present after Delete")
	}
	if _, ok := fake.m[id]; ok {
		t.Fatal("secret still present after Delete")
	}
	if s.Active() != "" {
		t.Fatalf("active = %q after deleting active connection", s.Active())
	}

	// Unknown UUID errors.
	if err := s.Delete("nope"); err == nil {
		t.Fatal("Delete of unknown UUID should error")
	}
	if err := s.Update(Connection{UUID: "nope"}, Credentials{}); err == nil {
		t.Fatal("Update of unknown UUID should error")
	}

	// Persistence: reload from disk.
	s2, err := NewStore(dir, newFake())
	if err != nil {
		t.Fatalf("reload: %v", err)
	}
	if len(s2.List()) != 1 || s2.List()[0].Name != "os" {
		t.Fatalf("reload List = %+v", s2.List())
	}
}

func TestStoreActive(t *testing.T) {
	s, _, _ := newTestStore(t)
	id1, _ := s.Add(Connection{Name: "a", AuthType: AuthNone}, Credentials{})
	id2, _ := s.Add(Connection{Name: "b", AuthType: AuthNone}, Credentials{})

	if s.Active() != id1 {
		t.Fatalf("first Add should become active, got %q", s.Active())
	}
	if err := s.SetActive(id2); err != nil {
		t.Fatalf("SetActive: %v", err)
	}
	if s.Active() != id2 {
		t.Fatalf("Active = %q, want %q", s.Active(), id2)
	}
	if _, ok := s.ActiveConnection(); !ok {
		t.Fatal("ActiveConnection missing")
	}
	if err := s.SetActive("nope"); err == nil {
		t.Fatal("SetActive of unknown UUID should error")
	}
	if _, ok := s.ActiveConnection(); ok && s.Active() == "" {
		t.Fatal("no active connection expected")
	}
}

func TestAtomicWritePermissions(t *testing.T) {
	s, _, dir := newTestStore(t)
	id, err := s.Add(Connection{Name: "x", URI: "http://x:9200", AuthType: AuthNone, Username: "u", Region: "r"}, Credentials{})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	path := filepath.Join(dir, "connections.json")
	fi, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if fi.Mode().Perm() != 0o600 {
		t.Fatalf("connections.json mode = %v, want 0600", fi.Mode().Perm())
	}
	// No leftover temp files.
	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if len(e.Name()) > 0 && e.Name()[0] == '.' {
			t.Fatalf("leftover temp file %q", e.Name())
		}
	}
	// Content is valid JSON with the connection and active UUID.
	raw, _ := os.ReadFile(path)
	var d struct {
		Connections []Connection `json:"connections"`
		Active      string       `json:"active"`
	}
	if err := json.Unmarshal(raw, &d); err != nil {
		t.Fatalf("file not valid JSON: %v", err)
	}
	if len(d.Connections) != 1 || d.Connections[0].UUID != id || d.Active != id {
		t.Fatalf("file content = %+v", d)
	}
}

func TestCredentialsPerAuthType(t *testing.T) {
	s, _, _ := newTestStore(t)

	id, _ := s.Add(Connection{Name: "aws", AuthType: AuthAWSIAM, Region: "us-east-1"},
		Credentials{SecretAccessKey: "s", AccessKeyID: "ak", SessionToken: "st", Password: "leak"})
	c := s.Credentials(id)
	if c.Password != "" || c.SecretAccessKey != "s" || c.AccessKeyID != "ak" || c.SessionToken != "st" {
		t.Fatalf("awsIAM creds = %+v", c)
	}

	id2, _ := s.Add(Connection{Name: "key", AuthType: AuthAPIKey}, Credentials{APIKey: "k", Password: "leak"})
	if c := s.Credentials(id2); c.APIKey != "k" || c.Password != "" {
		t.Fatalf("apiKey creds = %+v", c)
	}

	// ESAuth mapping.
	a := s.ESAuth(Connection{UUID: id, AuthType: AuthAWSIAM, Region: "us-east-1"})
	if a.Type != es.AuthAWSIAM || a.AccessKeyID != "ak" || a.SecretAccessKey != "s" ||
		a.SessionToken != "st" || a.Region != "us-east-1" {
		t.Fatalf("ESAuth = %+v", a)
	}
	a2 := s.ESAuth(Connection{UUID: id2, AuthType: AuthAPIKey})
	if a2.Password != "k" || a2.Type != es.AuthAPIKey {
		t.Fatalf("ESAuth apiKey = %+v", a2)
	}
}

func TestTestConnection(t *testing.T) {
	info := map[string]any{
		"cluster_name": "my cluster",
		"cluster_uuid": "abc-123",
		"name":         "node-1",
		"version": map[string]string{
			"number":       "8.14.1",
			"distribution": "",
			"build_flavor": "",
			"build_hash":   "dead",
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		json.NewEncoder(w).Encode(info)
	}))
	defer srv.Close()

	client, ci, err := TestConnection(context.Background(), srv.URL, es.Auth{Type: es.AuthNone}, nil)
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if ci.Version != "8.14.1" || ci.ClusterName != "my cluster" || ci.UUID != "abc-123" {
		t.Fatalf("ClusterInfo = %+v", ci)
	}
	if ci.Distribution != "elasticsearch" || ci.BuildFlavor != "default" {
		t.Fatalf("defaults not applied: %+v", ci)
	}
	if client.Version != "8.14.1" || client.Distribution != "elasticsearch" || client.BuildFlavor != "default" {
		t.Fatalf("client fields not set: %+v", client)
	}
}

func TestTestConnectionFallbacksAndServerless(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Old cluster: no cluster_uuid, no version fields beyond number.
		json.NewEncoder(w).Encode(map[string]any{
			"cluster_name": "old cluster",
			"name":         "Old Node",
			"version":      map[string]string{"number": "2.4.6", "distribution": "opensearch", "build_flavor": "serverless"},
		})
	}))
	defer srv.Close()

	_, ci, err := TestConnection(context.Background(), srv.URL, es.Auth{Type: es.AuthNone}, nil)
	if err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if ci.UUID != "oldcluster-OldNode" {
		t.Fatalf("fallback UUID = %q", ci.UUID)
	}
	if ci.Distribution != "opensearch" || ci.BuildFlavor != "serverless" {
		t.Fatalf("info = %+v", ci)
	}
}

func TestTestConnectionUnreachable(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }))
	url := srv.URL
	srv.Close() // also covers connection refused via second call below

	if _, _, err := TestConnection(context.Background(), url, es.Auth{Type: es.AuthNone}, nil); err == nil {
		t.Fatal("expected error for unreachable server")
	}
	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "boom", 500) }))
	defer srv2.Close()
	if _, _, err := TestConnection(context.Background(), srv2.URL, es.Auth{Type: es.AuthNone}, nil); err == nil {
		t.Fatal("expected error for 500 response")
	}
}
