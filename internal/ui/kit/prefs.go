package kit

import (
	"context"
	"encoding/json"
	"maps"
	"slices"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
)

// ColumnPrefs lazily loads one module table's column layout (order +
// widths) from the per-cluster SQLite prefs and keeps it in memory, saving
// it back when the table changes it. mygo's ui.Table mutates the
// ListState.Columns it is given in place (drag-resize rewrites Widths,
// drag-move rewrites Order) but has no column-change callback—its Changed
// covers selection and sort only—so modules lend the table a copy each
// frame and hand it back to Sync, which compares and persists.
type ColumnPrefs struct {
	loaded bool
	layout ui.TableLayout
}

// Frame returns the saved layout, loading it from the host's DB once.
func (p *ColumnPrefs) Frame(h ModuleHost, module string) ui.TableLayout {
	if !p.loaded {
		p.loaded = true
		p.layout = LoadColumns(h, module)
	}
	return p.layout
}

// Sync takes back the layout the table may have changed this frame, stores
// it in p and the DB, and reports whether it persisted the change.
func (p *ColumnPrefs) Sync(h ModuleHost, module string, frame ui.TableLayout) bool {
	if !p.loaded || sameLayout(p.layout, frame) {
		return false
	}
	p.layout = cloneLayout(frame)
	return SaveColumns(h, module, p.layout)
}

// LoadColumns returns the saved column layout for one cluster's module; a
// missing row, a closed DB or a bad payload all yield the zero layout, so
// the table starts with its built-in arrangement.
func LoadColumns(h ModuleHost, module string) ui.TableLayout {
	if h == nil || h.DB() == nil || h.ClusterKey() == "" {
		return ui.TableLayout{}
	}
	p, ok, err := h.DB().GetTablePrefs(context.Background(), h.ClusterKey(), module)
	if err != nil || !ok {
		return ui.TableLayout{}
	}
	var l ui.TableLayout
	if json.Unmarshal([]byte(p.ColumnsJSON), &l) != nil {
		return ui.TableLayout{}
	}
	return l
}

// SaveColumns persists the layout for one cluster's module and reports
// whether it wrote. With no cluster connected (empty ClusterKey) it is a
// no-op, so in-memory state still updates without inventing a phantom
// cluster row.
func SaveColumns(h ModuleHost, module string, l ui.TableLayout) bool {
	if h == nil || h.DB() == nil || h.ClusterKey() == "" {
		return false
	}
	b, err := json.Marshal(l)
	if err != nil {
		return false
	}
	if err := h.DB().SetTablePrefs(context.Background(), db.TablePrefs{ClusterUUID: h.ClusterKey(), Module: module, ColumnsJSON: string(b)}); err != nil {
		return false
	}
	return true
}

// sameLayout compares two layouts by value; nil and empty widths agree.
func sameLayout(a, b ui.TableLayout) bool {
	if !slices.Equal(a.Order, b.Order) {
		return false
	}
	if len(a.Widths) != len(b.Widths) {
		return false
	}
	for id, w := range a.Widths {
		if w2, ok := b.Widths[id]; !ok || w != w2 {
			return false
		}
	}
	return true
}

// cloneLayout deep-copies a layout so later table mutations cannot touch
// the held copy through shared slices or maps.
func cloneLayout(l ui.TableLayout) ui.TableLayout {
	return ui.TableLayout{
		Order:  slices.Clone(l.Order),
		Widths: maps.Clone(l.Widths),
	}
}
