package conn

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"seismic/internal/es"
)

// TestTLSInsecureSkipVerify reproduces the self-signed ES 8 docker case:
// https with an unknown-authority certificate fails by default and connects
// when the connection opts into skipping verification.
func TestTLSInsecureSkipVerify(t *testing.T) {
	srv := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"cluster_name":"tls-cluster","cluster_uuid":"u1","version":{"number":"8.14.1"}}`))
	}))
	defer srv.Close()
	uri := strings.Replace(srv.URL, "127.0.0.1", "localhost", 1)

	if _, _, err := TestConnection(context.Background(), uri, es.Auth{Type: es.AuthNone}, nil); err == nil {
		t.Fatal("self-signed TLS accepted without skip-verify")
	} else if !strings.Contains(err.Error(), "certificate") && !strings.Contains(err.Error(), "x509") {
		t.Fatalf("unexpected error: %v", err)
	}

	client, info, err := TestConnection(context.Background(), uri, es.Auth{Type: es.AuthNone}, TLSConfig(true))
	if err != nil {
		t.Fatalf("skip-verify connect: %v", err)
	}
	if info.ClusterName != "tls-cluster" || client.Version != "8.14.1" {
		t.Errorf("info = %+v", info)
	}
}

// TestTLSConfigNilForSecure covers the secure default.
func TestTLSConfigNilForSecure(t *testing.T) {
	if TLSConfig(false) != nil {
		t.Error("TLSConfig(false) must be nil")
	}
	if cfg := TLSConfig(true); cfg == nil || !cfg.InsecureSkipVerify {
		t.Error("TLSConfig(true) must set InsecureSkipVerify")
	}
}
