// Package recovery builds the「分片恢复」module page: _cat/recovery
// (newest first) with stage coloring and manual refresh.
package recovery

import (
	"context"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/ui/kit"
)

// Row is one _cat/recovery row.
type Row struct {
	Index     string `json:"index"`
	Shard     string `json:"shard"`
	Type      string `json:"type"`
	Stage     string `json:"stage"`
	Source    string `json:"source"`
	Node      string `json:"node"`
	StartTime string `json:"start_time"`
}

// stageColor: DONE Success, failed/aborted Danger, in-progress Warning —
// the theme's semantic colors.
func stageColor(t *ui.Theme, stage string) ui.Color {
	switch stage {
	case "DONE":
		return t.Success
	case "":
		return t.Danger
	default: // INDEX / VERIFY_INDEX / TRANSLOG / FINALIZE / INIT
		return t.Warning
	}
}

// Module holds the 分片恢复 module's state.
type Module struct {
	rows       []Row
	filter     string
	loading    bool
	err        error
	loadedOnce bool

	selectedRow int
	sort        ui.SortOrder
	prefs       kit.ColumnPrefs
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
				return strings.Compare(a.Type, b.Type)
			case "阶段":
				return strings.Compare(a.Stage, b.Stage)
			case "来源":
				return strings.Compare(a.Source, b.Source)
			case "节点":
				return strings.Compare(a.Node, b.Node)
			case "开始":
				return strings.Compare(a.StartTime, b.StartTime)
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

// load fetches _cat/recovery via the host's client (off-thread).
func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load("recovery",
		func() { m.loading = true; m.err = nil },
		func(ctx context.Context) func() {
			var rows []Row
			_, err := client.RequestJSON(ctx, "GET", "_cat/recovery", map[string]any{
				"format": "json",
				"s":      "start_time_millis:desc",
				"h":      "index,shard,type,stage,source,node,start_time",
			}, &rows)
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

func cols(lang string) []ui.TableColumn {
	return []ui.TableColumn{
		{Title: kit.Tr(lang, "索引"), Sortable: true},
		{Title: kit.Tr(lang, "分片"), Width: 60, Align: ui.End},
		{Title: kit.Tr(lang, "类型"), Width: 110, Sortable: true},
		{Title: kit.Tr(lang, "阶段"), Width: 140, Sortable: true},
		{Title: kit.Tr(lang, "来源"), Width: 150, Sortable: true},
		{Title: kit.Tr(lang, "节点"), Sortable: true},
		{Title: kit.Tr(lang, "开始"), Sortable: true},
	}
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	if !m.loadedOnce && !m.loading && h.ES() != nil {
		m.load(h)
	}
	t := c.Theme()
	// ⌘R 刷新，系统习惯。
	c.OnShortcut(ui.Cmd, ui.KeyR, func() { m.load(h) })

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(t.Space(4), t.Space(5), t.Space(2), t.Space(5)).Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, kit.Tr(h.Lang(), "分片恢复")).FontSize(kit.SizeTitle(t)).Bold()
				ui.Text(c, kit.Tr(h.Lang(), "{0} 条恢复记录", itoa(len(m.visibleRows())))).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Label(kit.Tr(h.Lang(), "筛选索引…")).Placeholder(kit.Tr(h.Lang(), "筛选索引…")).Width(220)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.load(h)
			}
		})

		if kit.StateText(c, h.Lang(), m.loading && len(m.rows) == 0, m.err, len(m.visibleRows()) == 0, kit.Tr(h.Lang(), "还没有恢复记录")) {
			return
		}

		rows := m.visibleRows()
		if m.selectedRow >= len(rows) {
			m.selectedRow = -1
		}
		listState := ui.ListState{Selected: &m.selectedRow, Sort: &m.sort, Columns: m.prefs.Frame(h, "recovery")}
		ui.Table(c, &listState, cols(h.Lang()), len(rows), func(row, col int) {
			r := rows[row]
			switch col {
			case 0:
				ui.Text(c, r.Index).SingleLine()
			case 1:
				ui.Text(c, r.Shard)
			case 2:
				ui.Text(c, r.Type)
			case 3:
				ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
					kit.Dot(c, t.Space(1.75), stageColor(t, r.Stage))
					ui.Text(c, r.Stage)
				})
			case 4:
				ui.Text(c, r.Source)
			case 5:
				ui.Text(c, r.Node)
			case 6:
				ui.Text(c, r.StartTime).TextColor(t.TextMuted)
			}
		}).Grow(1).Padding(0, t.Space(3), 0, t.Space(3))
		// ui.Table 在拖拽时原地改写 ListState.Columns（列变动没有专门
		// 回调），每帧把改完的布局交回 Sync 比较并持久化。
		m.prefs.Sync(h, "recovery", listState.Columns)
	})
}

func itoa(n int) string { return strconv.Itoa(n) }
