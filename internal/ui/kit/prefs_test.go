package kit

import (
	"context"
	"testing"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
)

type stubHost struct {
	db      *db.Store
	cluster string
}

func (s *stubHost) BuildFlavor() string                { return "" }
func (s *stubHost) ES() *es.Client                     { return nil }
func (s *stubHost) ClusterKey() string                 { return s.cluster }
func (s *stubHost) Version() string                    { return "" }
func (s *stubHost) Distribution() string               { return "elasticsearch" }
func (s *stubHost) DB() *db.Store                      { return s.db }
func (s *stubHost) Lang() string                       { return "zh" }
func (s *stubHost) Load(key string, mark func(), fn func(context.Context) func()) { mark(); fn(context.Background())() }

func newHost(t *testing.T) *stubHost {
	t.Helper()
	store, err := db.Open(context.Background(), t.TempDir()+"/prefs.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	return &stubHost{db: store, cluster: "c1"}
}

func TestColumnPrefsRoundTrip(t *testing.T) {
	h := newHost(t)

	var p ColumnPrefs
	if got := p.Frame(h, "indices"); got.Order != nil || got.Widths != nil {
		t.Fatalf("fresh module should start with zero layout, got %+v", got)
	}

	// The table changed the frame's layout: reorder + widen.
	frame := ui.TableLayout{
		Order:  []string{"状态", "索引", "文档数"},
		Widths: map[string]float32{"索引": 240, "文档数": 90},
	}
	if !p.Sync(h, "indices", frame) {
		t.Fatal("first sync after a change should report changed")
	}
	if p.Sync(h, "indices", frame) {
		t.Fatal("unchanged layout should not save again")
	}

	// A new module on the same cluster gets the saved layout back.
	var q ColumnPrefs
	got := q.Frame(h, "indices")
	if len(got.Order) != 3 || got.Order[0] != "状态" || got.Order[2] != "文档数" {
		t.Errorf("order not restored: %+v", got.Order)
	}
	if got.Widths["索引"] != 240 || got.Widths["文档数"] != 90 || len(got.Widths) != 2 {
		t.Errorf("widths not restored: %+v", got.Widths)
	}
}

func TestColumnPrefsIsolation(t *testing.T) {
	h := newHost(t)
	var p ColumnPrefs
	p.Frame(h, "nodes")
	frame := ui.TableLayout{Order: []string{"a", "b"}, Widths: map[string]float32{"a": 50}}
	p.Sync(h, "nodes", frame)
	// Mutating the frame afterwards must not leak into the held copy.
	frame.Order[0] = "zzz"
	frame.Widths["a"] = 999
	if held := p.Frame(h, "nodes"); held.Order[0] != "a" || held.Widths["a"] != 50 {
		t.Errorf("held layout mutated through shared slice/map: %+v", held)
	}
}

func TestLoadColumnsDegenerate(t *testing.T) {
	h := newHost(t)
	if got := LoadColumns(h, "shards"); got.Order != nil || got.Widths != nil {
		t.Errorf("missing row should give zero layout, got %+v", got)
	}
	// Bad payload falls back to the zero layout too.
	if err := h.db.SetTablePrefs(context.Background(), db.TablePrefs{
		ClusterUUID: "c1", Module: "shards", ColumnsJSON: "{not json",
	}); err != nil {
		t.Fatal(err)
	}
	if got := LoadColumns(h, "shards"); got.Order != nil || got.Widths != nil {
		t.Errorf("bad payload should give zero layout, got %+v", got)
	}
	// No cluster key: no-op save, and nothing was written.
	empty := &stubHost{db: h.db}
	SaveColumns(empty, "shards", ui.TableLayout{Order: []string{"x"}})
	if _, ok, _ := h.db.GetTablePrefs(context.Background(), "", "shards"); ok {
		t.Error("empty cluster key must not write a prefs row")
	}
	// nil DB host: no panic, zero layout.
	if got := LoadColumns(&stubHost{cluster: "c1"}, "shards"); got.Order != nil {
		t.Errorf("nil DB should give zero layout, got %+v", got)
	}
}
