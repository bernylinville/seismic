// Package templates builds the「索引模板」module page: the legacy
// _template and the composable _index_template endpoints side by side.
package templates

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// Module holds the 索引模板 module's state.
type Module struct {
	tab        int // 0 旧版, 1 组合
	loadedOnce bool
	loading    bool
	err        error

	filter      string
	selectedRow int // 选中的行号，-1 为无

	legacyRows []templateRow
	compRows   []templateRow
	composable bool // cluster supports _index_template (ES/OpenSearch >= 7)

	detail        []byte
	detailName    string
	detailLoading bool

	actionErr string

	// 删除确认对话框
	deleting   bool
	deleteName string

	// 新建对话框
	creating   bool
	createName string
	createPats string
	createBody string
	createErr  string
}

// New makes the module's state.
func New() *Module { return &Module{selectedRow: -1} }

// templateRow is one template row in either endpoint's list.
type templateRow struct {
	name     string
	order    string // order（旧版）或 priority（组合）
	patterns string
}

func templateCols(lang string) []ui.TableColumn {
	return []ui.TableColumn{
		{Title: kit.Tr(lang, "名称"), Sortable: true},
		{Title: kit.Tr(lang, "顺序"), Width: 90, Align: ui.End, Sortable: true},
		{Title: kit.Tr(lang, "索引模式"), Width: 260},
	}
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	client := h.ES()
	if client == nil {
		t := c.Theme()
		ui.Column(c).Fill().Padding(16).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "索引模板")).FontSize(20).Bold()
			ui.Text(c, kit.Tr(h.Lang(), "未连接集群")).TextColor(t.TextMuted)
		})
		return
	}
	if !m.loadedOnce && !m.loading {
		m.load(h)
	}
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 20, 8, 20).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, kit.Tr(h.Lang(), "索引模板")).FontSize(19).Bold()
				ui.Text(c, kit.Tr(h.Lang(), "{0} 个模板", len(m.visibleRows()))).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Placeholder(kit.Tr(h.Lang(), "筛选模板…")).Width(200)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.load(h)
			}
			if ui.Button(c, kit.Tr(h.Lang(), "新建…")).Clicked() {
				m.creating, m.createName, m.createPats, m.createBody, m.createErr = true, "", "", "{\n  \n}", ""
			}
			if m.selectedRow >= 0 && m.selectedRow < len(m.visibleRows()) {
				if ui.Button(c, kit.Tr(h.Lang(), "查看 JSON")).Clicked() {
					m.showDetail(h, m.visibleRows()[m.selectedRow])
				}
				if ui.Button(c, kit.Tr(h.Lang(), "删除…")).Clicked() {
					m.deleteName = m.visibleRows()[m.selectedRow].name
					m.deleting = true
				}
			}
		})

		if m.composable {
			ui.Tabs(c, &m.tab, kit.Tr(h.Lang(), "旧版模板"), kit.Tr(h.Lang(), "组合模板")).Padding(0, 20, 0, 8)
		}
		if m.actionErr != "" {
			ui.Text(c, m.actionErr).FontSize(12).TextColor(ui.Hex("#d64545")).Padding(0, 20, 4, 20)
		}
		if kit.StateText(c, h.Lang(), m.loading, m.err, len(m.rows()) == 0, kit.Tr(h.Lang(), "没有模板")) {
			return
		}

		rows := m.visibleRows()
		if m.selectedRow >= len(rows) {
			m.selectedRow = -1
		}
		state := ui.ListState{Selected: &m.selectedRow}
		table := ui.Table(c, &state, templateCols(h.Lang()), len(rows), func(row, col int) {
			r := rows[row]
			switch col {
			case 0:
				ui.Text(c, r.name).SingleLine()
			case 1:
				ui.Text(c, r.order).FontSize(12)
			case 2:
				ui.Text(c, r.patterns).FontSize(12).SingleLine().TextColor(t.TextMuted)
			}
		}).Grow(1).Padding(0, 12, 0, 12)
		if table.Submitted() {
			m.showDetail(h, rows[m.selectedRow])
		}

		if m.detailName != "" {
			m.detailView(c, h.Lang())
		}
	})

	m.deleteDialog(h, c)
	m.createDialog(h, c)
}

func (m *Module) rows() []templateRow {
	if m.composable && m.tab == 1 {
		return m.compRows
	}
	return m.legacyRows
}

func (m *Module) visibleRows() []templateRow {
	var out []templateRow
	for _, r := range m.rows() {
		if m.filter != "" && !strings.Contains(r.name, m.filter) {
			continue
		}
		out = append(out, r)
	}
	return out
}

func (m *Module) detailView(c *ui.Context, lang string) {
	t := c.Theme()
	ui.Column(c).Height(220).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
		Background(t.Surface).Padding(12, 16, 12, 16).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m.detailName).Bold()
			ui.Text(c, kit.Tr(lang, "· 定义 JSON")).TextColor(t.TextMuted).FontSize(12)
			ui.Spacer(c)
			if ui.Button(c, kit.Tr(lang, "关闭")).Clicked() {
				m.detailName, m.detail = "", nil
			}
		})
		if m.detailLoading {
			ui.Text(c, kit.Tr(lang, "加载中…")).TextColor(t.TextMuted)
			return
		}
		if len(m.detail) > 0 {
			ui.ScrollBoth(c).Grow(1).Children(func() {
				ui.Text(c, string(m.detail)).FontSize(12)
			})
		}
	})
}

func (m *Module) deleteDialog(h kit.ModuleHost, c *ui.Context) {
	if !m.deleting {
		return
	}
	t := c.Theme()
	ui.Modal(c, &m.deleting, func() {
		ui.Text(c, kit.Tr(h.Lang(), "删除模板")).Bold()
		ui.Text(c, kit.Tr(h.Lang(), "确定删除模板 {0} 吗？此操作不可撤销。", m.deleteName)).TextColor(t.TextMuted)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
				m.deleting = false
			}
			if ui.Button(c, kit.Tr(h.Lang(), "删除")).Clicked() {
				name := m.deleteName
				m.deleting = false
				m.run(h, "删除", func(ctx context.Context, client *es.Client) error {
					_, err := client.Request(ctx, "DELETE", m.endpoint(h)+"/"+es.CleanIndexName(name), nil)
					return err
				})
			}
		})
	})
}

func (m *Module) createDialog(h kit.ModuleHost, c *ui.Context) {
	if !m.creating {
		return
	}
	ui.Modal(c, &m.creating, func() {
		ui.Text(c, kit.Tr(h.Lang(), "新建模板")).Bold()
		ui.Column(c).Gap(8).Children(func() {
			ui.TextInput(c, &m.createName).Label(kit.Tr(h.Lang(), "名称"))
			ui.TextInput(c, &m.createPats).Label(kit.Tr(h.Lang(), "索引模式（逗号分隔）"))
			ui.TextArea(c, &m.createBody).Label(kit.Tr(h.Lang(), "JSON 主体")).Lines(3, 10)
			if m.createErr != "" {
				ui.Text(c, m.createErr).FontSize(12).TextColor(ui.Hex("#d64545"))
			}
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
					m.creating = false
				}
				if ui.PrimaryButton(c, kit.Tr(h.Lang(), "创建")).Clicked() {
					m.create(h)
				}
			})
		})
	})
}

// endpoint is the active tab's path, "_template" or "_index_template".
func (m *Module) endpoint(h kit.ModuleHost) string {
	if m.composable && m.tab == 1 {
		return "_index_template"
	}
	return "_template"
}

// create validates the dialog's fields and PUTs the new template.
func (m *Module) create(h kit.ModuleHost) {
	if strings.TrimSpace(m.createName) == "" || strings.TrimSpace(m.createPats) == "" {
		m.createErr = kit.Tr(h.Lang(), "名称和索引模式不能为空")
		return
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(m.createBody), &body); err != nil {
		m.createErr = kit.Tr(h.Lang(), "JSON 主体无效: {0}", err.Error())
		return
	}
	patterns := []string{}
	for _, p := range strings.Split(m.createPats, ",") {
		if p = strings.TrimSpace(p); p != "" {
			patterns = append(patterns, p)
		}
	}
	if body == nil {
		body = map[string]any{}
	}
	body["index_patterns"] = patterns
	if m.endpoint(h) == "_index_template" {
		// 组合模板的 settings/mappings 等包在 template 里。
		inner := map[string]any{}
		for _, k := range []string{"template", "composed_of", "priority", "version", "_meta", "data_stream"} {
			if v, ok := body[k]; ok {
				inner[k] = v
			}
		}
		body = inner
		body["index_patterns"] = patterns
	}

	name := m.createName
	payload := body
	m.creating = false
	m.run(h, "新建", func(ctx context.Context, client *es.Client) error {
		_, err := client.Request(ctx, "PUT", m.endpoint(h)+"/"+es.CleanIndexName(name), payload)
		return err
	})
}

// showDetail loads one template's full JSON definition.
func (m *Module) showDetail(h kit.ModuleHost, row templateRow) {
	client := h.ES()
	if client == nil {
		return
	}
	m.detailName = row.name
	h.Load(
		func() { m.detailLoading = true },
		func() func() {
			resp, err := client.Request(context.Background(), "GET", m.endpoint(h)+"/"+es.CleanIndexName(row.name), nil)
			return func() {
				m.detailLoading = false
				if err != nil {
					m.actionErr = kit.ErrMessage(err)
					m.detail, m.detailName = nil, ""
					return
				}
				m.detail = resp.Body
			}
		},
	)
}

// run performs an action off-thread, then reloads the list.
func (m *Module) run(h kit.ModuleHost, name string, fn func(ctx context.Context, client *es.Client) error) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load(
		func() { m.actionErr = "" },
		func() func() {
			err := fn(context.Background(), client)
			return func() {
				if err != nil {
					m.actionErr = kit.Tr(h.Lang(), "{0}失败: {1}", kit.Tr(h.Lang(), name), kit.ErrMessage(err))
				}
				m.load(h)
			}
		},
	)
}

// load fetches both endpoint's template lists (composable only on 7+).
func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	m.composable = es.VersionAtLeast(h.Version(), "7.0.0")
	h.Load(
		func() { m.loading = true },
		func() func() {
			legacyRows, legacyErr := fetchTemplates(func() (*es.Response, error) { return client.Template(context.Background()) })
			var compRows []templateRow
			var compErr error
			if m.composable {
				compRows, compErr = fetchComposable(func() (*es.Response, error) { return client.IndexTemplate(context.Background()) })
			}
			return func() {
				m.loading = false
				m.loadedOnce = true
				switch {
				case legacyErr != nil:
					m.err = legacyErr
				case compErr != nil:
					m.err = compErr
				default:
					m.err = nil
					m.legacyRows = legacyRows
					m.compRows = compRows
				}
			}
		},
	)
}

// fetchTemplates parses the legacy _template response: a name→definition map.
func fetchTemplates(get func() (*es.Response, error)) ([]templateRow, error) {
	resp, err := get()
	if err != nil {
		return nil, err
	}
	var defs map[string]json.RawMessage
	if err := resp.JSON(&defs); err != nil {
		return nil, err
	}
	rows := make([]templateRow, 0, len(defs))
	for name, raw := range defs {
		var def struct {
			Order         int      `json:"order"`
			IndexPatterns []string `json:"index_patterns"`
		}
		if err := json.Unmarshal(raw, &def); err != nil {
			return nil, err
		}
		rows = append(rows, templateRow{
			name:     name,
			order:    fmt.Sprintf("%d", def.Order),
			patterns: strings.Join(def.IndexPatterns, ", "),
		})
	}
	sortRows(rows)
	return rows, nil
}

// fetchComposable parses the _index_template response.
func fetchComposable(get func() (*es.Response, error)) ([]templateRow, error) {
	resp, err := get()
	if err != nil {
		return nil, err
	}
	var body struct {
		IndexTemplates []struct {
			Name          string   `json:"name"`
			Priority      *int     `json:"priority"`
			IndexPatterns []string `json:"index_patterns"`
		} `json:"index_templates"`
	}
	if err := resp.JSON(&body); err != nil {
		return nil, err
	}
	rows := make([]templateRow, 0, len(body.IndexTemplates))
	for _, t := range body.IndexTemplates {
		order := ""
		if t.Priority != nil {
			order = fmt.Sprintf("%d", *t.Priority)
		}
		rows = append(rows, templateRow{
			name:     t.Name,
			order:    order,
			patterns: strings.Join(t.IndexPatterns, ", "),
		})
	}
	sortRows(rows)
	return rows, nil
}

func sortRows(rows []templateRow) {
	sort.Slice(rows, func(i, j int) bool { return rows[i].name < rows[j].name })
}
