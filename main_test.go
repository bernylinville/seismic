package main

import (
	"testing"

	"github.com/egoist/mygo/ui"

	appui "seismic/internal/ui"
)

// TestShellNavigates builds the window without a display and walks the
// sidebar: the welcome page first, the module pages once connected.
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
