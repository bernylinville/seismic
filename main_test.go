package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"seismic/internal/conn"
	"seismic/internal/db"
	appui "seismic/internal/ui"
)

// newES serves the ES endpoints the connection flow touches.
func newES(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.Write([]byte(`{"cluster_name":"test-cluster","cluster_uuid":"abc123","version":{"number":"8.14.1","distribution":"elasticsearch","build_flavor":"default"}}`))
		case r.URL.Path == "/_cluster/health":
			w.Write([]byte(`{"status":"green"}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestShellNavigates(t *testing.T) {
	a := appui.NewApp(ui.NewRouter("/overview"))
	tt := ui.NewTester(a.View, 1280, 800)

	if !tt.HasText("Seismic") || !tt.HasText("添加连接") {
		t.Fatalf("welcome page missing, texts %q", tt.Texts())
	}

	a.Connected = true
	a.ClusterName = "prod-es-8"
	a.ClusterHealth = "green"
	a.ClusterVer = "8.14.1"
	tt.Frame()

	if !tt.HasText("prod-es-8") {
		t.Errorf("cluster name missing from toolbar, texts %q", tt.Texts())
	}
	for _, label := range []string{"概览", "索引", "REST 控制台", "用户", "角色", "快照仓库"} {
		if err := tt.Click(label); err != nil {
			t.Fatalf("click %s: %v", label, err)
		}
	}
	if a.Router().Path() != "/repositories" {
		t.Errorf("router path = %s, want /repositories", a.Router().Path())
	}
}

func TestConnectionFlow(t *testing.T) {
	es := newES(t)
	dir := t.TempDir()

	secrets, err := conn.NewFileSecretStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	conns, err := conn.NewStore(dir, secrets)
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	a := appui.NewApp(ui.NewRouter("/overview"))
	a.AttachDeps(appui.Deps{Conns: conns, DB: database, HomeDir: dir})
	tt := ui.NewTester(a.View, 1280, 800)

	if !tt.HasText("添加连接") {
		t.Fatalf("form missing, texts %q", tt.Texts())
	}
	a.SetConnectForm("测试集群", es.URL, "none")
	if err := tt.Click("保存并连接"); err != nil {
		t.Fatalf("click save: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for !a.Connected && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !a.Connected {
		t.Fatalf("not connected; testMsg=%q", a.ConnectTestMessage())
	}
	tt.Frame()
	if !tt.HasText("测试集群") || !tt.HasText("8.14.1") {
		t.Errorf("cluster info missing from window, texts %q", tt.Texts())
	}

	// The saved connection dials again on startup.
	a2 := appui.NewApp(ui.NewRouter("/overview"))
	a2.AttachDeps(appui.Deps{Conns: conns, DB: database, HomeDir: dir})
	deadline = time.Now().Add(3 * time.Second)
	for !a2.Connected && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !a2.Connected {
		t.Fatal("saved connection did not reconnect")
	}
}

func TestShellEnglishUI(t *testing.T) {
	es := newES(t)
	dir := t.TempDir()
	secrets, err := conn.NewFileSecretStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	conns, err := conn.NewStore(dir, secrets)
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(context.Background(), filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	a := appui.NewApp(ui.NewRouter("/indices"))
	a.SetLang("en")
	a.AttachDeps(appui.Deps{Conns: conns, DB: database, HomeDir: dir})
	tt := ui.NewTester(a.View, 1280, 800)
	if !tt.HasText("Add connection") {
		t.Fatalf("english form missing, texts %q", tt.Texts())
	}
	a.SetConnectForm("测试集群", es.URL, "none")
	if err := tt.Click("Save & Connect"); err != nil {
		t.Fatalf("click save: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !a.Connected && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !a.Connected {
		t.Fatalf("not connected; testMsg=%q", a.ConnectTestMessage())
	}
	time.Sleep(50 * time.Millisecond)
	tt.Frame()
	for _, want := range []string{"Overview", "Indices", "REST Console", "Cluster", "0 indices", "Hide system indices"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q, texts %q", want, tt.Texts())
		}
	}
}
