package ui

import (
	"context"
	"testing"

	"seismic/internal/db"
)

// TestIndicesColumnPrefsPersisted checks the indices table's column layout
// survives an app restart: the frame the table mutated goes through Sync
// into the per-cluster DB, and a fresh App loads it back.
func TestIndicesColumnPrefsPersisted(t *testing.T) {
	store, err := db.Open(context.Background(), t.TempDir()+"/indices.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })

	a1 := &App{lang: "zh"}
	a1.deps.DB = store
	a1.clusterUUID = "c1"

	frame := a1.indices.prefs.Frame(a1, "indices")
	if frame.Order != nil || frame.Widths != nil {
		t.Fatalf("fresh app should start with zero layout, got %+v", frame)
	}
	// What ui.Table would leave in ListState.Columns after a drag.
	frame.Order = []string{"健康", "索引", "状态", "文档数", "存储大小"}
	frame.Widths = map[string]float32{"索引": 260, "文档数": 90}
	if !a1.indices.prefs.Sync(a1, "indices", frame) {
		t.Fatal("changed layout should save")
	}

	// A new app on the same cluster restores order and widths.
	a2 := &App{lang: "zh"}
	a2.deps.DB = store
	a2.clusterUUID = "c1"
	got := a2.indices.prefs.Frame(a2, "indices")
	if len(got.Order) != 5 || got.Order[0] != "健康" || got.Order[4] != "存储大小" {
		t.Errorf("column order not restored: %+v", got.Order)
	}
	if got.Widths["索引"] != 260 || got.Widths["文档数"] != 90 || len(got.Widths) != 2 {
		t.Errorf("column widths not restored: %+v", got.Widths)
	}

	// A different cluster starts clean, and nothing was written for it.
	a3 := &App{lang: "zh"}
	a3.deps.DB = store
	a3.clusterUUID = "c2"
	if g := a3.indices.prefs.Frame(a3, "indices"); g.Order != nil {
		t.Errorf("another cluster should start empty, got %+v", g)
	}
	if _, ok, _ := store.GetTablePrefs(context.Background(), "c2", "indices"); ok {
		t.Error("no prefs row should exist for c2")
	}

	// Not connected (empty cluster key): saving is a no-op.
	a4 := &App{lang: "zh"}
	a4.deps.DB = store
	a4.indices.prefs.Frame(a4, "indices")
	if a4.indices.prefs.Sync(a4, "indices", frame) {
		t.Error("sync without a cluster should not report saved")
	}
}
