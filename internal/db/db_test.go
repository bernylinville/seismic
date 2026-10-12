package db

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

func openTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	s, err := Open(ctx, filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestHistoryLimitOffsetOrderAndClear(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()
	base := time.Now().UnixMilli()
	for i := range 5 {
		if _, err := s.AddHistory(ctx, HistoryEntry{
			ClusterUUID: "c1", Method: "GET", Path: "/_cat/indices",
			Status: 200, DurationMS: int64(i + 1), CreatedAt: base + int64(i),
		}); err != nil {
			t.Fatalf("AddHistory: %v", err)
		}
	}
	// cluster isolation: another cluster's entries must be invisible
	if _, err := s.AddHistory(ctx, HistoryEntry{ClusterUUID: "c2", Method: "GET", Path: "/other", Status: 200, CreatedAt: base + 99}); err != nil {
		t.Fatalf("AddHistory c2: %v", err)
	}

	all, err := s.ListHistory(ctx, "c1", 100, 0)
	if err != nil {
		t.Fatalf("ListHistory: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("want 5 entries, got %d", len(all))
	}
	for i, e := range all {
		want := base + int64(4-i)
		if e.CreatedAt != want {
			t.Fatalf("entry %d: created_at = %d, want %d (DESC order)", i, e.CreatedAt, want)
		}
		if e.ClusterUUID != "c1" || e.Method != "GET" || e.Status != 200 || e.DurationMS != int64(5-i) {
			t.Fatalf("entry %d roundtrip mismatch: %+v", i, e)
		}
		if e.ID == 0 {
			t.Fatalf("entry %d missing id", i)
		}
	}

	page, err := s.ListHistory(ctx, "c1", 2, 1)
	if err != nil {
		t.Fatalf("ListHistory paged: %v", err)
	}
	if len(page) != 2 || page[0].CreatedAt != base+3 || page[1].CreatedAt != base+2 {
		t.Fatalf("limit/offset wrong: %+v", page)
	}

	if err := s.ClearHistory(ctx, "c1"); err != nil {
		t.Fatalf("ClearHistory: %v", err)
	}
	if left, _ := s.ListHistory(ctx, "c1", 100, 0); len(left) != 0 {
		t.Fatalf("c1 history not cleared")
	}
	if other, _ := s.ListHistory(ctx, "c2", 100, 0); len(other) != 1 {
		t.Fatalf("ClearHistory leaked across clusters")
	}
}

func TestSaveQueryUpsertAndIsolation(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	id1, err := s.SaveQuery(ctx, SavedQuery{ClusterUUID: "c1", Name: "q", Method: "GET", Path: "/a", Body: "{}"})
	if err != nil {
		t.Fatalf("SaveQuery insert: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	id2, err := s.SaveQuery(ctx, SavedQuery{ClusterUUID: "c1", Name: "q", Method: "POST", Path: "/b", Body: `{"x":1}`})
	if err != nil {
		t.Fatalf("SaveQuery upsert: %v", err)
	}
	if id1 != id2 {
		t.Fatalf("upsert changed id: %d -> %d", id1, id2)
	}
	// same name in another cluster is a separate row
	id3, err := s.SaveQuery(ctx, SavedQuery{ClusterUUID: "c2", Name: "q", Method: "GET", Path: "/c2"})
	if err != nil {
		t.Fatalf("SaveQuery c2: %v", err)
	}
	if id3 == id1 {
		t.Fatalf("different clusters share row")
	}
	if _, err := s.SaveQuery(ctx, SavedQuery{ClusterUUID: "c1", Name: "other", Method: "GET", Path: "/z"}); err != nil {
		t.Fatalf("SaveQuery second name: %v", err)
	}

	qs, err := s.ListQueries(ctx, "c1")
	if err != nil {
		t.Fatalf("ListQueries: %v", err)
	}
	if len(qs) != 2 {
		t.Fatalf("c1 want 2 queries, got %d", len(qs))
	}
	var got SavedQuery
	for _, q := range qs {
		if q.Name == "q" {
			got = q
		}
	}
	if got.Method != "POST" || got.Path != "/b" || got.Body != `{"x":1}` {
		t.Fatalf("upsert did not update fields: %+v", got)
	}
	if got.CreatedAt == 0 || got.UpdatedAt < got.CreatedAt {
		t.Fatalf("timestamps wrong: %+v", got)
	}

	if err := s.DeleteQuery(ctx, "c1", got.ID); err != nil {
		t.Fatalf("DeleteQuery: %v", err)
	}
	if qs, _ = s.ListQueries(ctx, "c1"); len(qs) != 1 {
		t.Fatalf("delete failed")
	}
	// c1 cannot delete c2's row by id
	if err := s.DeleteQuery(ctx, "c1", id3); err != nil {
		t.Fatalf("DeleteQuery cross cluster: %v", err)
	}
	if qs, _ = s.ListQueries(ctx, "c2"); len(qs) != 1 {
		t.Fatalf("cross-cluster delete leaked")
	}
}

func TestTablePrefsUpsert(t *testing.T) {
	s := openTestStore(t)
	ctx := context.Background()

	if _, ok, err := s.GetTablePrefs(ctx, "c1", "indices"); err != nil || ok {
		t.Fatalf("expected miss, ok=%v err=%v", ok, err)
	}
	if err := s.SetTablePrefs(ctx, TablePrefs{ClusterUUID: "c1", Module: "indices", ColumnsJSON: `["name","status"]`}); err != nil {
		t.Fatalf("SetTablePrefs: %v", err)
	}
	if err := s.SetTablePrefs(ctx, TablePrefs{ClusterUUID: "c1", Module: "indices", ColumnsJSON: `["status"]`}); err != nil {
		t.Fatalf("SetTablePrefs upsert: %v", err)
	}
	p, ok, err := s.GetTablePrefs(ctx, "c1", "indices")
	if err != nil || !ok {
		t.Fatalf("expected hit, ok=%v err=%v", ok, err)
	}
	if p.ColumnsJSON != `["status"]` {
		t.Fatalf("upsert value wrong: %+v", p)
	}
	if _, ok, _ := s.GetTablePrefs(ctx, "c2", "indices"); ok {
		t.Fatalf("prefs leaked across clusters")
	}
}
