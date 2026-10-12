package snapshots

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

// TestSnapshotsColumnPrefsPersisted checks both snapshot tables (the list
// and the SLM policy table) restore their column layouts in a new module.
func TestSnapshotsColumnPrefsPersisted(t *testing.T) {
	store, err := db.Open(context.Background(), t.TempDir()+"/snapshots.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	host := dbHost{fakeHost: &fakeHost{}, store: store, key: "test-cluster"}

	m1 := New()
	listFrame := m1.prefs.Frame(host, "snapshots")
	listFrame.Order = []string{"状态", "快照", "开始", "结束"}
	listFrame.Widths = map[string]float32{"快照": 200}
	if !m1.prefs.Sync(host, "snapshots", listFrame) {
		t.Fatal("changed snapshot-list layout should save")
	}
	slmFrame := m1.slmPrefs.Frame(host, "snapshots.slm")
	slmFrame.Widths = map[string]float32{"名称": 120}
	if !m1.slmPrefs.Sync(host, "snapshots.slm", slmFrame) {
		t.Fatal("changed SLM layout should save")
	}

	m2 := New()
	got := m2.prefs.Frame(host, "snapshots")
	if len(got.Order) != 4 || got.Order[0] != "状态" || got.Order[1] != "快照" {
		t.Errorf("snapshot column order not restored: %+v", got.Order)
	}
	if got.Widths["快照"] != 200 {
		t.Errorf("snapshot column width not restored: %+v", got.Widths)
	}
	slm := m2.slmPrefs.Frame(host, "snapshots.slm")
	if slm.Widths["名称"] != 120 || len(slm.Widths) != 1 {
		t.Errorf("SLM column widths not restored: %+v", slm.Widths)
	}

	// Both rows live in the DB under distinct module keys.
	for _, mod := range []string{"snapshots", "snapshots.slm"} {
		if p, ok, err := store.GetTablePrefs(context.Background(), "test-cluster", mod); err != nil || !ok || p.ColumnsJSON == "" {
			t.Errorf("GetTablePrefs(%q): ok=%v err=%v", mod, ok, err)
		}
	}
}
