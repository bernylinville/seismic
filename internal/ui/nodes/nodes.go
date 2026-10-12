// Package nodes builds the「节点」module page.
package nodes

import (
	"cmp"
	"context"
	"encoding/json"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/ui/kit"
)

// hideAttrsRegex is elasticvue's DEFAULT_HIDE_NODE_ATTRIBUTES_REGEX: system
// attributes (ml./xpack./transform.) stay out of the detail until asked for.
var hideAttrsRegex = regexp.MustCompile(`^(ml|xpack|transform)\.`)

// Module holds the 节点 module's state.
type Module struct {
	rows       []NodeRow
	filter     string
	hideAttrs  bool
	loading    bool
	err        error
	loadedOnce bool

	selectedRow int // 选中的行号，-1 为无
	sort        ui.SortOrder

	detailName string
	detailJSON string
	detailHide bool
	detailLoad bool
	detailErr  error
	// nodeCache 缓存 _nodes 返回的每个节点对象；displayCache 缓存按
	// (节点, 是否隐藏系统属性) 渲染好的 JSON。
	nodeCache    map[string]map[string]any
	displayCache map[displayKey]string
}

// New makes the module's state.
func New() *Module {
	return &Module{
		hideAttrs:    true,
		selectedRow:  -1,
		nodeCache:    map[string]map[string]any{},
		displayCache: map[displayKey]string{},
	}
}

type displayKey struct {
	name string
	hide bool
}

// NodeRow is one _cat/nodes row; cat 端点有的版本把数值列返回成字符串，
// flexString 两种都吃。
type NodeRow struct {
	Name        flexString `json:"name"`
	IP          flexString `json:"ip"`
	Role        flexString `json:"role"`
	HeapPercent flexString `json:"heap.percent"`
	RamPercent  flexString `json:"ram.percent"`
	CPU         flexString `json:"cpu"`
	Load1m      flexString `json:"load_1m"`
	DiskPercent flexString `json:"disk.used_percent"`
	NodeRole    flexString `json:"node.role"`
	Version     flexString `json:"version"`
}

type flexString string

// UnmarshalJSON accepts both "41" and 41 from cat endpoints.
func (f *flexString) UnmarshalJSON(b []byte) error {
	if len(b) > 0 && b[0] == '"' {
		var s string
		if err := json.Unmarshal(b, &s); err != nil {
			return err
		}
		*f = flexString(s)
		return nil
	}
	*f = flexString(strings.TrimSpace(string(b)))
	return nil
}

func (f flexString) String() string { return string(f) }

func (m *Module) visibleRows() []NodeRow {
	out := make([]NodeRow, 0, len(m.rows))
	for _, r := range m.rows {
		if m.filter != "" && !filterMatch(r, m.filter) {
			continue
		}
		out = append(out, r)
	}
	slices.SortStableFunc(out, func(a, b NodeRow) int {
		res := cmp.Compare(a.Name.String(), b.Name.String())
		switch m.sort.Column {
		case "堆内存%":
			res = numeric(a.HeapPercent, b.HeapPercent)
		case "内存%":
			res = numeric(a.RamPercent, b.RamPercent)
		case "CPU":
			res = numeric(a.CPU, b.CPU)
		case "负载 1m":
			res = numeric(a.Load1m, b.Load1m)
		case "磁盘%":
			res = numeric(a.DiskPercent, b.DiskPercent)
		case "版本":
			res = strings.Compare(a.Version.String(), b.Version.String())
		}
		if m.sort.Descending {
			return -res
		}
		return res
	})
	return out
}

func filterMatch(r NodeRow, filter string) bool {
	return strings.Contains(r.Name.String(), filter) ||
		strings.Contains(r.IP.String(), filter) ||
		strings.Contains(r.Version.String(), filter)
}

// numeric compares two cat cells as floats, falling back to text.
func numeric(a, b flexString) int {
	fa, ea := strconv.ParseFloat(a.String(), 64)
	fb, eb := strconv.ParseFloat(b.String(), 64)
	if ea == nil && eb == nil {
		return cmp.Compare(fa, fb)
	}
	return strings.Compare(a.String(), b.String())
}

// load fetches the node table.
func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load(
		func() { m.loading = true },
		func() func() {
			rows, err := func() ([]NodeRow, error) {
				params := map[string]any{
					"format": "json",
					"h":      "name,ip,role,heap.percent,ram.percent,cpu,load_1m,disk.used_percent,node.role,version",
				}
				resp, err := client.CatNodes(context.Background(), params)
				if err != nil {
					return nil, err
				}
				var rows []NodeRow
				if err := resp.JSON(&rows); err != nil {
					return nil, err
				}
				return rows, nil
			}()
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

// loadDetail fetches a node's full info from _nodes, cached per node.
func (m *Module) loadDetail(h kit.ModuleHost, name string) {
	if node, ok := m.nodeCache[name]; ok {
		m.detailName, m.detailJSON = name, m.renderDetail(node)
		m.detailLoad, m.detailErr = false, nil
		return
	}
	client := h.ES()
	if client == nil {
		return
	}
	m.detailName, m.detailJSON = name, ""
	m.detailLoad, m.detailErr = true, nil
	h.Load(
		func() {},
		func() func() {
			nodes, err := func() (map[string]map[string]any, error) {
				resp, err := client.Nodes(context.Background())
				if err != nil {
					return nil, err
				}
				var body struct {
					Nodes map[string]map[string]any `json:"nodes"`
				}
				if err := resp.JSON(&body); err != nil {
					return nil, err
				}
				return body.Nodes, nil
			}()
			return func() {
				m.detailLoad, m.detailErr = false, err
				if err != nil {
					return
				}
				byName := make(map[string]map[string]any, len(nodes))
				for _, node := range nodes {
					byName[nodeName(node)] = node
				}
				m.nodeCache = byName
				if node, ok := byName[name]; ok {
					m.detailJSON = m.renderDetail(node)
				}
			}
		},
	)
}

// renderDetail marshals a node's info, hiding system attributes when asked.
func (m *Module) renderDetail(node map[string]any) string {
	key := displayKey{name: nodeName(node), hide: m.hideAttrs}
	if s, ok := m.displayCache[key]; ok {
		return s
	}
	out := make(map[string]any, len(node)+1)
	for k, v := range node {
		out[k] = v
	}
	if attrs, ok := node["attributes"].(map[string]any); ok && m.hideAttrs {
		kept := map[string]any{}
		for k, v := range attrs {
			if hideAttrsRegex.MatchString(k) {
				continue
			}
			kept[k] = v
		}
		out["attributes"] = kept
	}
	b, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return ""
	}
	m.displayCache[key] = string(b)
	return string(b)
}

func nodeName(node map[string]any) string {
	name, _ := node["name"].(string)
	return name
}

var nodesCols = []ui.TableColumn{
	{Title: "名称", Sortable: true},
	{Title: "IP", Width: 130},
	{Title: "角色", Width: 70},
	{Title: "堆内存%", Width: 80, Align: ui.End, Sortable: true},
	{Title: "内存%", Width: 70, Align: ui.End, Sortable: true},
	{Title: "CPU", Width: 60, Align: ui.End, Sortable: true},
	{Title: "负载 1m", Width: 80, Align: ui.End, Sortable: true},
	{Title: "磁盘%", Width: 70, Align: ui.End, Sortable: true},
	{Title: "角色详情", Width: 170},
	{Title: "版本", Width: 80},
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	if !m.loadedOnce && !m.loading && h.ES() != nil {
		m.load(h)
	}

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 20, 8, 20).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, "节点").FontSize(19).Bold()
				ui.Text(c, strconv.Itoa(len(m.visibleRows()))+" 个节点").FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Placeholder("筛选节点…").Width(200)
			ui.Checkbox(c, &m.hideAttrs, "隐藏系统属性")
			if ui.Button(c, "刷新").Clicked() {
				m.load(h)
			}
		})

		if kit.StateText(c, m.loading, m.err, len(m.rows) == 0, "没有节点") {
			return
		}

		rows := m.visibleRows()
		if m.selectedRow >= len(rows) {
			m.selectedRow = -1
		}
		listState := ui.ListState{Selected: &m.selectedRow, Sort: &m.sort}
		ui.Table(c, &listState, nodesCols, len(rows), func(row, col int) {
			r := rows[row]
			switch col {
			case 0:
				ui.Text(c, r.Name.String()).SingleLine()
			case 1:
				ui.Text(c, r.IP.String()).FontSize(12)
			case 2:
				ui.Text(c, r.Role.String()).FontSize(12)
			case 3:
				ui.Text(c, r.HeapPercent.String()+"%").FontSize(12)
			case 4:
				ui.Text(c, r.RamPercent.String()+"%").FontSize(12)
			case 5:
				ui.Text(c, r.CPU.String()+"%").FontSize(12)
			case 6:
				ui.Text(c, r.Load1m.String()).FontSize(12)
			case 7:
				ui.Text(c, r.DiskPercent.String()+"%").FontSize(12)
			case 8:
				ui.Text(c, r.NodeRole.String()).FontSize(12).TextColor(t.TextMuted)
			case 9:
				ui.Text(c, r.Version.String()).FontSize(12)
			}
		}).Grow(1).Padding(0, 12, 0, 12)

		if m.selectedRow >= 0 && m.selectedRow < len(rows) {
			name := rows[m.selectedRow].Name.String()
			if m.detailName != name || m.detailHide != m.hideAttrs {
				if node, ok := m.nodeCache[name]; ok {
					m.detailName, m.detailHide = name, m.hideAttrs
					m.detailJSON = m.renderDetail(node)
				} else {
					m.detailHide = m.hideAttrs
					m.loadDetail(h, name)
				}
			}
			m.detailView(c)
		}
	})
}

func (m *Module) detailView(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Height(220).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
		Background(t.Surface).Padding(12, 16, 12, 16).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m.detailName).Bold()
			ui.Text(c, "· 节点详情").TextColor(t.TextMuted).FontSize(12)
			ui.Spacer(c)
			if ui.Button(c, "关闭").Clicked() {
				m.selectedRow = -1
				m.detailName, m.detailJSON = "", ""
			}
		})
		switch {
		case m.detailLoad:
			ui.Text(c, "加载中…").TextColor(t.TextMuted)
		case m.detailErr != nil:
			ui.Text(c, kit.ErrMessage(m.detailErr)).FontSize(12).TextColor(ui.Hex("#d64545"))
		default:
			ui.Text(c, m.detailJSON).FontSize(12).Grow(1)
		}
	})
}
