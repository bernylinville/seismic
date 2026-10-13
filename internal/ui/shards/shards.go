// Package shards builds the「分片」module page: _cat/shards table with
// state coloring, filter, and per-shard reroute via _cluster/reroute.
package shards

import (
	"context"
	"encoding/json"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/ui/kit"
)

// Row is one _cat/shards row (h=index,shard,prirep,state,docs,store,node,unassigned.reason).
type Row struct {
	Index      string      `json:"index"`
	Shard      json.Number `json:"shard"`
	Prirep     string      `json:"prirep"`
	State      string      `json:"state"`
	Docs       json.Number `json:"docs"`
	Store      string      `json:"store"`
	Node       string      `json:"node"`
	Unassigned string      `json:"unassigned.reason"`
}

// stateColor maps shard state to the app's three semantic colors.
func stateColor(state string) ui.Color {
	switch state {
	case "STARTED":
		return ui.Hex("#2f9e63")
	case "UNASSIGNED":
		return ui.Hex("#d64545")
	default: // INITIALIZING / RELOCATING / POST_RECOVERY …
		return ui.Hex("#d9a406")
	}
}

// Module holds the 分片 module's state.
type Module struct {
	rows       []Row
	filter     string
	loading    bool
	err        error
	loadedOnce bool

	selectedRow int
	sort        ui.SortOrder
	prefs       kit.ColumnPrefs

	// rerouting is the shard awaiting a target-node pick, nil 为无.
	rerouting *Row
	actionErr string
}

// New makes the module's state.
func New() *Module { return &Module{selectedRow: -1} }

func (m *Module) visibleRows() []Row {
	f := strings.ToLower(m.filter)
	rows := make([]Row, 0, len(m.rows))
	for _, r := range m.rows {
		if f != "" && !strings.Contains(strings.ToLower(r.Index), f) {
			continue
		}
		rows = append(rows, r)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		cmp := func(a, b Row) int {
			switch m.sort.Column {
			case "类型":
				return strings.Compare(a.Prirep, b.Prirep)
			case "状态":
				return strings.Compare(a.State, b.State)
			case "文档数":
				return strings.Compare(a.Docs.String(), b.Docs.String())
			case "大小":
				return strings.Compare(a.Store, b.Store)
			case "节点":
				return strings.Compare(a.Node, b.Node)
			default: // 索引
				return strings.Compare(a.Index, b.Index)
			}
		}
		c := cmp(rows[i], rows[j])
		if m.sort.Descending {
			c = -c
		}
		return c < 0
	})
	return rows
}

// nodes lists the distinct nodes present in the table, as reroute targets.
func (m *Module) nodes() []string {
	seen := map[string]bool{}
	var out []string
	for _, r := range m.rows {
		if r.Node != "" && !seen[r.Node] {
			seen[r.Node] = true
			out = append(out, r.Node)
		}
	}
	sort.Strings(out)
	return out
}

// load fetches _cat/shards via the host's client (off-thread).
func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load(
		func() { m.loading = true; m.err = nil },
		func() func() {
			var rows []Row
			_, err := client.CatShardsInto(context.Background(), map[string]any{
				"format": "json",
				"h":      "index,shard,prirep,state,docs,store,node,unassigned.reason",
			}, m.filterIndex(), &rows)
			return func() {
				m.loading = false
				m.loadedOnce = true
				m.err = err
				if err == nil {
					m.rows = rows
				}
			}
		},
	)
}

// filterIndex turns the filter into the _cat/shards/<filter>* path segment.
// Only a leading index-name prefix is a valid path filter; anything with
// spaces falls back to filtering client-side after a full load.
func (m *Module) filterIndex() string {
	if m.filter == "" || strings.ContainsAny(m.filter, " /*?\"\\") {
		return ""
	}
	return m.filter
}

// reroute sends the _cluster/reroute command for the pending shard toward
// the picked node, then reloads. Command shapes mirror elasticvue's
// clusterReroute: move for assigned shards; allocate_replica /
// allocate_stale_primary for unassigned ones.
func (m *Module) reroute(h kit.ModuleHost, shard Row, targetNode string) {
	client := h.ES()
	if client == nil || targetNode == "" {
		return
	}
	var cmd map[string]any
	switch {
	case shard.Node != "":
		cmd = map[string]any{"move": map[string]any{
			"index": shard.Index, "shard": shard.Shard.String(),
			"from_node": shard.Node, "to_node": targetNode,
		}}
	case shard.Prirep == "r":
		cmd = map[string]any{"allocate_replica": map[string]any{
			"index": shard.Index, "shard": shard.Shard.String(), "node": targetNode,
		}}
	default:
		cmd = map[string]any{"allocate_stale_primary": map[string]any{
			"index": shard.Index, "shard": shard.Shard.String(), "node": targetNode,
			"allow_primary": true,
		}}
	}
	h.Load(
		func() { m.actionErr = "" },
		func() func() {
			_, err := client.ClusterReroute(context.Background(), []any{cmd})
			return func() {
				if err != nil {
					m.actionErr = kit.Tr(h.Lang(), "重新分配失败：{0}", kit.ErrMessage(err))
					return
				}
				m.rerouting = nil
				m.load(h)
			}
		},
	)
}

func cols(lang string) []ui.TableColumn {
	return []ui.TableColumn{
		{Title: kit.Tr(lang, "索引"), Sortable: true},
		{Title: kit.Tr(lang, "分片"), Width: 60, Align: ui.End},
		{Title: kit.Tr(lang, "类型"), Width: 70, Sortable: true},
		{Title: kit.Tr(lang, "状态"), Width: 130, Sortable: true},
		{Title: kit.Tr(lang, "文档数"), Width: 100, Align: ui.End, Sortable: true},
		{Title: kit.Tr(lang, "大小"), Width: 90, Align: ui.End, Sortable: true},
		{Title: kit.Tr(lang, "节点"), Sortable: true},
		{Title: kit.Tr(lang, "原因"), Width: 150},
	}
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	if !m.loadedOnce && !m.loading && h.ES() != nil {
		m.load(h)
	}
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 20, 8, 20).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, kit.Tr(h.Lang(), "分片")).FontSize(19).Bold()
				ui.Text(c, kit.Tr(h.Lang(), "{0} 个分片", itoa(len(m.visibleRows())))).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Label(kit.Tr(h.Lang(), "筛选索引…")).Placeholder(kit.Tr(h.Lang(), "筛选索引…")).Width(220)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.load(h)
			}
		})

		if m.actionErr != "" {
			ui.Text(c, m.actionErr).FontSize(12).TextColor(ui.Hex("#d64545")).Padding(0, 20, 4, 20)
		}

		if m.rerouting != nil {
			m.rerouteBar(h, c)
		}

		if kit.StateText(c, h.Lang(), m.loading && len(m.rows) == 0, m.err, len(m.visibleRows()) == 0, kit.Tr(h.Lang(), "还没有分片")) {
			return
		}

		rows := m.visibleRows()
		if m.selectedRow >= len(rows) {
			m.selectedRow = -1
		}
		listState := ui.ListState{Selected: &m.selectedRow, Sort: &m.sort, Columns: m.prefs.Frame(h, "shards")}
		ui.Table(c, &listState, cols(h.Lang()), len(rows), func(row, col int) {
			r := rows[row]
			switch col {
			case 0:
				ui.Text(c, r.Index).SingleLine().ContextMenu(func(menu *ui.Menu) {
					if r.Node != "" {
						if menu.Item(kit.Tr(h.Lang(), "重新分配…")).Chosen() {
							m.rerouting = &r
						}
					} else {
						if menu.Item(kit.Tr(h.Lang(), "移到节点…")).Chosen() {
							m.rerouting = &r
						}
					}
				})
			case 1:
				ui.Text(c, r.Shard.String()).FontSize(12)
			case 2:
				ui.Text(c, kit.Tr(h.Lang(), prirepLabel(r.Prirep))).FontSize(12)
			case 3:
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					kit.Dot(c, 7, stateColor(r.State))
					ui.Text(c, r.State).FontSize(12)
				})
			case 4:
				ui.Text(c, r.Docs.String()).FontSize(12)
			case 5:
				ui.Text(c, r.Store).FontSize(12)
			case 6:
				ui.Text(c, r.Node).FontSize(12)
			case 7:
				ui.Text(c, r.Unassigned).FontSize(12).TextColor(t.TextMuted)
			}
		}).Grow(1).Padding(0, 12, 0, 12)
		// ui.Table 在拖拽时原地改写 ListState.Columns（列变动没有专门
		// 回调），每帧把改完的布局交回 Sync 比较并持久化。
		m.prefs.Sync(h, "shards", listState.Columns)
	})
}

// rerouteBar renders the target-node picker for the pending reroute.
func (m *Module) rerouteBar(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	shard := m.rerouting
	ui.Row(c).Padding(8, 20).Gap(8).AlignItems(ui.Center).Children(func() {
		if shard.Node != "" {
			ui.Text(c, kit.Tr(h.Lang(), "重新分配 {0}[{1}]（当前 {2}）到：", shard.Index, shard.Shard.String(), shard.Node)).FontSize(12)
		} else {
			ui.Text(c, kit.Tr(h.Lang(), "移动 {0}[{1}] 到：", shard.Index, shard.Shard.String())).FontSize(12)
		}
		for _, node := range m.nodes() {
			if node == shard.Node {
				continue
			}
			if ui.Button(c, node).Clicked() {
				m.reroute(h, *shard, node)
				m.rerouting = nil
			}
		}
		ui.Spacer(c)
		if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
			m.rerouting = nil
		}
	}).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
}

func prirepLabel(p string) string {
	if p == "p" {
		return "主"
	}
	return "副本"
}

func itoa(n int) string { return strconv.Itoa(n) }
