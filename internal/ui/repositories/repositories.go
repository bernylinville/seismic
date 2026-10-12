// Package repositories builds the「快照仓库」module page: the _snapshot
// repository list, create (fs/s3/url + settings JSON), verify and delete.
package repositories

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// RepoRow is one _snapshot repository entry.
type RepoRow struct {
	Name     string
	Type     string
	Settings map[string]any
}

// repoEntry mirrors one _snapshot repository body.
type repoEntry struct {
	Type     string         `json:"type"`
	Settings map[string]any `json:"settings"`
}

// Module holds the 快照仓库 module's state.
type Module struct {
	rows       []RepoRow
	filter     string
	loading    bool
	err        error
	loadedOnce bool
	selected   int

	actionErr string
	actionMsg string

	// 新建仓库对话框。
	showNew  bool
	newName  string
	newType  string
	newJSON  string
	newErr   string
	newFocus bool

	// 删除确认对话框。
	deleteName    string
	deleteConfirm bool
}

// New makes the module's state.
func New() *Module {
	return &Module{selected: -1, newType: "fs", newJSON: settingsTemplate("fs")}
}

var repoCols = []ui.TableColumn{
	{Title: "名称", Sortable: true},
	{Title: "类型", Width: 90},
	{Title: "设置"},
	{Title: "操作", Width: 130},
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	client := h.ES()
	if client != nil && !m.loadedOnce && !m.loading {
		m.load(h)
	}
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 20, 8, 20).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, "快照仓库").FontSize(19).Bold()
				ui.Text(c, fmt.Sprintf("%d 个仓库", len(m.rows))).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Placeholder("筛选仓库…").Width(200)
			if ui.Button(c, "刷新").Clicked() {
				m.load(h)
			}
			if ui.PrimaryButton(c, "新建仓库…").Clicked() {
				m.showNew, m.newName, m.newErr = true, "", ""
				m.newJSON = settingsTemplate(m.newType)
				m.newFocus = true
			}
		})

		if m.actionErr != "" {
			ui.Text(c, m.actionErr).FontSize(12).TextColor(ui.Hex("#d64545")).Padding(0, 20, 4, 20)
		}
		if m.actionMsg != "" {
			ui.Text(c, m.actionMsg).FontSize(12).TextColor(t.TextMuted).Padding(0, 20, 4, 20)
		}
		if client == nil {
			ui.Text(c, "未连接集群").TextColor(t.TextMuted).Padding(0, 16)
			return
		}
		if kit.StateText(c, m.loading && !m.loadedOnce, m.err, len(m.visibleRows()) == 0, "没有仓库") {
			m.dialogs(h, c)
			return
		}

		rows := m.visibleRows()
		if m.selected >= len(rows) {
			m.selected = -1
		}
		listState := ui.ListState{Selected: &m.selected, Key: func(i int) any { return rows[i].Name }}
		ui.Table(c, &listState, repoCols, len(rows), func(row, col int) {
			r := rows[row]
			switch col {
			case 0:
				ui.Text(c, r.Name).SingleLine()
			case 1:
				ui.Text(c, r.Type).FontSize(12)
			case 2:
				ui.Text(c, settingsSummary(r.Settings)).FontSize(12).TextColor(t.TextMuted).SingleLine()
			case 3:
				ui.Row(c).Gap(6).Children(func() {
					if ui.Button(c, "验证").Clicked() {
						m.actionErr, m.actionMsg = "", ""
						m.verify(h, r.Name)
					}
					if ui.Button(c, "删除…").Clicked() {
						m.deleteName, m.deleteConfirm = r.Name, true
					}
				})
			}
		}).Grow(1).Padding(0, 12, 0, 12)

		m.dialogs(h, c)
	})
}

func (m *Module) dialogs(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()

	ui.Modal(c, &m.showNew, func() {
		ui.Text(c, "新建快照仓库").Bold()
		ui.Form(c, func() {
			ui.Field(c, "仓库名称", func() {
				input := ui.TextInput(c, &m.newName)
				if m.newFocus {
					input.AutoFocus()
					m.newFocus = false
				}
			})
			ui.Field(c, "类型", func() {
				if ui.Select(c, &m.newType, []string{"fs", "s3", "url"}).Changed() {
					m.newJSON = settingsTemplate(m.newType)
				}
			})
			ui.Field(c, "设置 JSON", func() {
				ui.TextInput(c, &m.newJSON)
			}).Description("仓库 settings，如 " + strings.Join(strings.Fields(settingsTemplate(m.newType)), " "))
		})
		if m.newErr != "" {
			ui.Text(c, m.newErr).FontSize(12).TextColor(ui.Hex("#d64545"))
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Clicked() {
				m.showNew = false
			}
			if ui.PrimaryButton(c, "创建").Clicked() {
				m.create(h)
			}
		})
	})

	if m.deleteConfirm {
		name := m.deleteName
		ui.Modal(c, &m.deleteConfirm, func() {
			ui.Text(c, "删除仓库").Bold()
			ui.Text(c, fmt.Sprintf("确定删除仓库 %s？其快照将不可用。", name)).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					m.deleteName, m.deleteConfirm = "", false
				}
				if ui.PrimaryButton(c, "删除").Clicked() {
					m.delete(h)
				}
			})
		})
	}
}

// visibleRows filters by name, sorted by name.
func (m *Module) visibleRows() []RepoRow {
	var out []RepoRow
	for _, r := range m.rows {
		if m.filter != "" && !strings.Contains(r.Name, m.filter) {
			continue
		}
		out = append(out, r)
	}
	slices.SortStableFunc(out, func(a, b RepoRow) int { return strings.Compare(a.Name, b.Name) })
	return out
}

func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	run(h, func() { m.loading = true }, func() ([]RepoRow, error) {
		resp, err := client.CatRepositories(context.Background(), nil)
		if err != nil {
			return nil, err
		}
		var entries map[string]repoEntry
		if err := resp.JSON(&entries); err != nil {
			return nil, err
		}
		rows := make([]RepoRow, 0, len(entries))
		for name, e := range entries {
			rows = append(rows, RepoRow{Name: name, Type: e.Type, Settings: e.Settings})
		}
		return rows, nil
	}, func(rows []RepoRow, err error) {
		m.loading, m.loadedOnce = false, true
		if err != nil {
			m.err = err
			return
		}
		m.err, m.rows = nil, rows
	})
}

func (m *Module) create(h kit.ModuleHost) {
	if m.newName == "" {
		m.newErr = "名称不能为空"
		return
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(m.newJSON), &settings); err != nil {
		m.newErr = "settings JSON 无效: " + err.Error()
		return
	}
	client := h.ES()
	if client == nil {
		return
	}
	body := map[string]any{"type": m.newType, "settings": settings}
	m.newErr = ""
	run(h, func() {}, func() (*es.Response, error) {
		return client.SnapshotCreateRepository(context.Background(), m.newName, body)
	}, func(_ *es.Response, err error) {

		if err == nil {
			m.showNew = false
			m.load(h)
			return
		}
		m.newErr = kit.ErrMessage(err)
	})
}

func (m *Module) verify(h kit.ModuleHost, name string) {
	client := h.ES()
	if client == nil {
		return
	}
	run(h, func() { m.loading = true }, func() (*es.Response, error) {
		return client.Request(context.Background(), "GET", "_snapshot/"+name+"/_verify", nil)
	}, func(_ *es.Response, err error) {
		m.loading = false
		if err == nil {
			m.actionMsg = name + " 验证通过"
			return
		}
		m.actionErr = kit.ErrMessage(err)
	})
}

func (m *Module) delete(h kit.ModuleHost) {
	client := h.ES()
	name := m.deleteName
	if client == nil || name == "" {
		return
	}

	run(h, func() { m.actionErr, m.actionMsg = "", "" }, func() (*es.Response, error) {
		return client.SnapshotDeleteRepository(context.Background(), name)
	}, func(_ *es.Response, err error) {

		if err == nil {
			m.deleteName, m.deleteConfirm = "", false
			m.load(h)
			return
		}
		m.actionErr = kit.ErrMessage(err)
	})
}

// run wraps h.Load: fn runs off-thread, the returned closure applies its
// result on the main thread.
func run[T any](h kit.ModuleHost, mark func(), fn func() (T, error), apply func(T, error)) {
	h.Load(mark, func() func() {
		v, err := fn()
		return func() {
			apply(v, err)
		}
	})
}

// settingsTemplate is the prefilled settings JSON for a repository type.
func settingsTemplate(typ string) string {
	switch typ {
	case "s3":
		return "{\n  \"bucket\": \"\",\n  \"client\": \"default\",\n  \"region\": \"\"\n}"
	case "url":
		return "{\n  \"url\": \"http://example.com/snapshots\"\n}"
	default:
		return "{\n  \"location\": \"/mnt/snapshots\"\n}"
	}
}

// settingsSummary compacts a settings map to one line, keys sorted.
func settingsSummary(settings map[string]any) string {
	if len(settings) == 0 {
		return ""
	}
	keys := slices.Sorted(maps.Keys(settings))
	var b strings.Builder
	b.WriteByte('{')
	for i, k := range keys {
		if i > 0 {
			b.WriteString(", ")
		}
		v, _ := json.Marshal(settings[k])
		b.WriteString(k)
		b.WriteString(": ")
		b.Write(v)
	}
	b.WriteByte('}')
	return b.String()
}
