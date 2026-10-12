// Package conn stores saved cluster connections and their credentials,
// a Go port of elasticvue's src/store/connection.ts. Connection metadata
// lives in dir/connections.json (0600); secrets live in a SecretStore
// (macOS Keychain, or dir/credentials.json elsewhere).
package conn

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"seismic/internal/es"
)

// Auth type strings, mirroring elasticvue's AuthType enum.
const (
	AuthNone   = "none"
	AuthBasic  = "basicAuth"
	AuthAPIKey = "apiKey"
	AuthAWSIAM = "awsIAM"
)

// Connection is one saved cluster. Credentials are kept separately in the
// SecretStore; only non-secret auth parameters live here.
type Connection struct {
	UUID     string `json:"uuid"`
	Name     string `json:"name"`
	URI      string `json:"uri"`
	AuthType string `json:"authType"` // none | basicAuth | apiKey | awsIAM
	Username string `json:"username"` // basicAuth
	Region   string `json:"region"`   // awsIAM

	// TLSInsecure skips certificate verification for self-signed clusters
	// (the ES 8 docker default turns http TLS on). Explicit user choice.
	TLSInsecure bool `json:"tlsInsecure,omitempty"`
}

// Credentials are the secret halves of a connection; which fields matter
// depends on Connection.AuthType.
type Credentials struct {
	Password        string `json:"password"`        // basicAuth (or raw API key), apiKey
	APIKey          string `json:"apiKey"`          // apiKey
	AccessKeyID     string `json:"accessKeyId"`     // awsIAM
	SecretAccessKey string `json:"secretAccessKey"` // awsIAM
	SessionToken    string `json:"sessionToken"`    // awsIAM
}

// ClusterInfo is the parsed GET / payload, aligned with elasticvue's
// connect() in src/composables/ClusterConnection.ts.
type ClusterInfo struct {
	Version      string // version.number, e.g. "8.14.1"
	Distribution string // version.distribution, default "elasticsearch"
	BuildFlavor  string // version.build_flavor, default "default"
	ClusterName  string // cluster_name
	UUID         string // cluster_uuid (with the pre-5 fallback)
}

// TLSConfig returns the TLS settings for a connection: nil keeps system
// verification; insecure clusters opt into InsecureSkipVerify (ES 8 docker
// ships a self-signed http certificate by default).
func TLSConfig(insecure bool) *tls.Config {
	if !insecure {
		return nil
	}
	return &tls.Config{InsecureSkipVerify: true}
}

// SecretStore holds per-connection secrets keyed by the connection UUID.
type SecretStore interface {
	Set(account, secret string) error
	Get(account string) string
	Delete(account string) error
}

type storeData struct {
	Connections []Connection `json:"connections"`
	Active      string       `json:"active"`
}

// Store persists connections in dir/connections.json and secrets in the
// injected SecretStore.
type Store struct {
	mu      sync.Mutex
	dir     string
	secrets SecretStore
	data    storeData
}

// NewStore loads (or initializes) the connection store in dir.
func NewStore(dir string, secrets SecretStore) (*Store, error) {
	if secrets == nil {
		return nil, errors.New("conn: nil SecretStore")
	}
	s := &Store{dir: dir, secrets: secrets}
	if err := s.Load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) file() string { return filepath.Join(s.dir, "connections.json") }

// Load reads connections.json; a missing file starts empty.
func (s *Store) Load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	raw, err := os.ReadFile(s.file())
	if errors.Is(err, os.ErrNotExist) {
		s.data = storeData{}
		return nil
	}
	if err != nil {
		return err
	}
	var d storeData
	if err := json.Unmarshal(raw, &d); err != nil {
		return fmt.Errorf("conn: parse %s: %w", s.file(), err)
	}
	s.data = d
	return nil
}

// Save writes connections.json atomically (temp file + rename) with 0600.
func (s *Store) Save() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.saveLocked()
}

func (s *Store) saveLocked() error {
	raw, err := json.MarshalIndent(s.data, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(s.dir, 0o700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(s.dir, ".connections-*.json")
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
	return os.Rename(name, s.file())
}

// List returns a copy of all connections.
func (s *Store) List() []Connection {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Connection, len(s.data.Connections))
	copy(out, s.data.Connections)
	return out
}

// Get returns the connection with the given UUID.
func (s *Store) Get(uuid string) (Connection, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.data.Connections {
		if c.UUID == uuid {
			return c, true
		}
	}
	return Connection{}, false
}

func newUUID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("conn: crypto/rand unavailable: " + err.Error())
	}
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	h := hex.EncodeToString(b[:])
	return h[0:8] + "-" + h[8:12] + "-" + h[12:16] + "-" + h[16:20] + "-" + h[20:32]
}

// setSecrets merges the used credential fields into the SecretStore. The
// secret is one JSON blob so the Keychain keeps a single item per
// connection; fields left empty never overwrite existing values.
func (s *Store) setSecrets(c Connection, creds Credentials) {
	creds = usedCredentials(c, creds)
	if creds == (Credentials{}) {
		return
	}
	var cur Credentials
	if raw := s.secrets.Get(c.UUID); raw != "" {
		_ = json.Unmarshal([]byte(raw), &cur)
	}
	if creds.Password != "" {
		cur.Password = creds.Password
	}
	if creds.APIKey != "" {
		cur.APIKey = creds.APIKey
	}
	if creds.AccessKeyID != "" {
		cur.AccessKeyID = creds.AccessKeyID
	}
	if creds.SecretAccessKey != "" {
		cur.SecretAccessKey = creds.SecretAccessKey
	}
	if creds.SessionToken != "" {
		cur.SessionToken = creds.SessionToken
	}
	raw, err := json.Marshal(cur)
	if err != nil {
		return
	}
	_ = s.secrets.Set(c.UUID, string(raw))
}

// Add stores a new connection (generating its UUID) and its credentials,
// persists, and returns the UUID. Empty credential fields are ignored so
// updating a connection never wipes an unchanged secret.
func (s *Store) Add(c Connection, creds Credentials) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	c.UUID = newUUID()
	s.data.Connections = append(s.data.Connections, c)
	if s.data.Active == "" {
		s.data.Active = c.UUID
	}
	if err := s.saveLocked(); err != nil {
		return "", err
	}
	s.setSecrets(c, creds)
	return c.UUID, nil
}

// Update replaces the stored connection with c (c.UUID identifies it) and
// refreshes any non-empty credential fields.
func (s *Store) Update(c Connection, creds Credentials) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.data.Connections {
		if s.data.Connections[i].UUID == c.UUID {
			s.data.Connections[i] = c
			if err := s.saveLocked(); err != nil {
				return err
			}
			s.setSecrets(c, creds)
			return nil
		}
	}
	return fmt.Errorf("conn: connection %q not found", c.UUID)
}

// Delete removes the connection and its stored secret. Deleting the active
// connection clears the active UUID.
func (s *Store) Delete(uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range s.data.Connections {
		if c.UUID == uuid {
			s.data.Connections = append(s.data.Connections[:i], s.data.Connections[i+1:]...)
			if s.data.Active == uuid {
				s.data.Active = ""
			}
			if err := s.saveLocked(); err != nil {
				return err
			}
			_ = s.secrets.Delete(uuid)
			return nil
		}
	}
	return fmt.Errorf("conn: connection %q not found", uuid)
}

// Credentials returns the stored secrets for a connection.
func (s *Store) Credentials(uuid string) Credentials {
	var creds Credentials
	if raw := s.secrets.Get(uuid); raw != "" {
		_ = json.Unmarshal([]byte(raw), &creds)
	}
	return creds
}

// usedCredentials zeroes the fields irrelevant to the connection's auth
// type, mirroring elasticvue's cleanupClusterAuth.
func usedCredentials(c Connection, creds Credentials) Credentials {
	switch c.AuthType {
	case AuthBasic:
		return Credentials{Password: creds.Password}
	case AuthAPIKey:
		return Credentials{APIKey: creds.APIKey}
	case AuthAWSIAM:
		return Credentials{
			AccessKeyID:     creds.AccessKeyID,
			SecretAccessKey: creds.SecretAccessKey,
			SessionToken:    creds.SessionToken,
		}
	default:
		return Credentials{}
	}
}

// SetActive marks the currently selected connection.
func (s *Store) SetActive(uuid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, c := range s.data.Connections {
		if c.UUID == uuid {
			s.data.Active = uuid
			return s.saveLocked()
		}
	}
	return fmt.Errorf("conn: connection %q not found", uuid)
}

// Active returns the active connection UUID, "" when none.
func (s *Store) Active() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.data.Active
}

// ActiveConnection returns the active connection, if any.
func (s *Store) ActiveConnection() (Connection, bool) {
	return s.Get(s.Active())
}

// ESAuth builds the es.Auth for a stored connection.
func (s *Store) ESAuth(c Connection) es.Auth {
	creds := s.Credentials(c.UUID)
	a := es.Auth{Type: es.AuthType(c.AuthType)}
	switch c.AuthType {
	case AuthBasic:
		a.Username = c.Username
		a.Password = creds.Password
	case AuthAPIKey:
		a.Password = creds.APIKey
	case AuthAWSIAM:
		a.AccessKeyID = creds.AccessKeyID
		a.SecretAccessKey = creds.SecretAccessKey
		a.SessionToken = creds.SessionToken
		a.Region = c.Region
	}
	return a
}

// TestConnection dials uri, pings GET /, and returns the client with its
// Version/Distribution/BuildFlavor set plus the parsed cluster info —
// elasticvue's adapter.test() followed by connect()'s info extraction.
func TestConnection(ctx context.Context, uri string, auth es.Auth, tlsCfg *tls.Config) (*es.Client, *ClusterInfo, error) {
	client, err := es.NewWithTLS(uri, auth, tlsCfg)
	if err != nil {
		return nil, nil, err
	}
	resp, err := client.Request(ctx, "GET", "", nil)
	if err != nil {
		return nil, nil, err
	}
	var info struct {
		ClusterName string `json:"cluster_name"`
		ClusterUUID string `json:"cluster_uuid"`
		Name        string `json:"name"` // pre-5 fallback part
		Version     struct {
			Number       string `json:"number"`
			Distribution string `json:"distribution"`
			BuildFlavor  string `json:"build_flavor"`
		} `json:"version"`
	}
	if err := resp.JSON(&info); err != nil {
		return nil, nil, fmt.Errorf("conn: parse cluster info: %w", err)
	}

	ci := &ClusterInfo{
		Version:      info.Version.Number,
		Distribution: info.Version.Distribution,
		BuildFlavor:  info.Version.BuildFlavor,
		ClusterName:  info.ClusterName,
	}
	if ci.Distribution == "" {
		ci.Distribution = "elasticsearch"
	}
	if ci.BuildFlavor == "" {
		ci.BuildFlavor = "default"
	}
	if ci.UUID = info.ClusterUUID; ci.UUID == "" {
		// elasticvue fallback for clusters older than 5.x
		ci.UUID = strings.ReplaceAll(info.ClusterName+"-"+info.Name, " ", "")
	}

	client.Version = ci.Version
	client.Distribution = ci.Distribution
	client.BuildFlavor = ci.BuildFlavor
	return client, ci, nil
}
