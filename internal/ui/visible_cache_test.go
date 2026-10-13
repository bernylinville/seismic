package ui

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestIndicesVisibleRowsCached checks the visible-rows cache: repeated calls
// reuse the computed slice while rows/filter/hideSystem/sort are unchanged,
// and any input change recomputes.
func TestIndicesVisibleRowsCached(t *testing.T) {
	s := &IndicesState{selectedRow: -1}
	s.rows = []IndexRow{
		{Index: "logs-2", DocsCount: "5"},
		{Index: ".internal", DocsCount: "1"},
		{Index: "logs-1", DocsCount: "9"},
	}

	r1 := s.visibleRows()
	r2 := s.visibleRows()
	if len(r1) != 3 || r1[0].Index != ".internal" || r1[1].Index != "logs-1" {
		t.Fatalf("visibleRows = %+v", r1)
	}
	if &r1[0] != &r2[0] {
		t.Error("unchanged inputs should reuse the cached slice")
	}

	s.filter = "logs"
	if got := s.visibleRows(); len(got) != 2 {
		t.Errorf("filter change not applied: %+v", got)
	}

	s.filter = ""
	s.hideSystem = true
	if got := s.visibleRows(); len(got) != 2 || got[0].Index == ".internal" {
		t.Errorf("hideSystem change not applied: %+v", got)
	}

	s.sort = ui.SortOrder{Column: "docs", Descending: true}
	if got := s.visibleRows(); got[0].Index != "logs-1" {
		t.Errorf("sort change not applied: %+v", got)
	}

	// A replaced rows slice (same length, new backing array) must recompute.
	s.sort = ui.SortOrder{}
	s.rows = []IndexRow{{Index: "zzz"}, {Index: "only"}, {Index: "aaa"}}
	if got := s.visibleRows(); got[0].Index != "aaa" {
		t.Errorf("rows swap left a stale cache: %+v", got)
	}
}
