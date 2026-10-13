package users

import (
	"context"
	"testing"

	"seismic/internal/db"
)

// dbHost gives the fake host a real SQLite store.
type dbHost struct {
	*fakeHost
	store *db.Store
	key   string // ClusterKey override
}

func (d dbHost) DB() *db.Store      { return d.store }
func (d dbHost) ClusterKey() string { return d.key }

// TestUsersColumnPrefsPersisted checks the users table's column layout
// survives a module restart: the frame the table mutated goes through
// Sync into the per-cluster DB, and a new module loads it back.
func TestUsersColumnPrefsPersisted(t *testing.T) {
	store, err := db.Open(context.Background(), t.TempDir()+"/users.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	host := dbHost{fakeHost: &fakeHost{lang: "zh"}, store: store, key: "test-cluster"}

	m1 := New()
	frame := m1.prefs.Frame(host, "users")
	if frame.Order != nil || frame.Widths != nil {
		t.Fatalf("fresh module should start with zero layout, got %+v", frame)
	}
	// What ui.Table would leave in ListState.Columns after a drag.
	frame.Order = []string{"用户名", "角色", "全名", "邮箱", "启用", "系统保留"}
	frame.Widths = map[string]float32{"用户名": 220, "角色": 160}
	if !m1.prefs.Sync(host, "users", frame) {
		t.Fatal("changed layout should save")
	}

	// The DB row really holds the JSON for this cluster and module.
	p, ok, err := store.GetTablePrefs(context.Background(), "test-cluster", "users")
	if err != nil || !ok {
		t.Fatalf("GetTablePrefs: ok=%v err=%v", ok, err)
	}
	if p.ColumnsJSON == "" {
		t.Fatal("empty columns_json")
	}

	m2 := New()
	got := m2.prefs.Frame(host, "users")
	if len(got.Order) != 6 || got.Order[0] != "用户名" || got.Order[5] != "系统保留" {
		t.Errorf("column order not restored: %+v", got.Order)
	}
	if got.Widths["用户名"] != 220 || got.Widths["角色"] != 160 || len(got.Widths) != 2 {
		t.Errorf("column widths not restored: %+v", got.Widths)
	}

	other := dbHost{fakeHost: &fakeHost{lang: "zh"}, store: store, key: "c2"}
	if g := (New()).prefs.Frame(other, "users"); g.Order != nil {
		t.Errorf("another cluster should start empty, got %+v", g)
	}
}
