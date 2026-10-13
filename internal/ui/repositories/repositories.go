// Package repositories builds the「快照仓库」module page: the _snapshot
// repository list, create (fs/s3/url + settings JSON), verify and delete.
package repositories

import (
	"context"
	"encoding/json"
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
	prefs      kit.ColumnPrefs

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

func repoCols(lang string) []ui.TableColumn {
	return []ui.TableColumn{
		{Title: kit.Tr(lang, "名称"), Sortable: true},
		{Title: kit.Tr(lang, "类型"), Width: 90},
		{Title: kit.Tr(lang, "设置")},
		{Title: kit.Tr(lang, "操作"), Width: 130},
	}
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	client := h.ES()
	if client != nil && !m.loadedOnce && !m.loading {
		m.load(h)
	}
	t := c.Theme()
	// ⌘R 刷新，系统习惯。
	c.OnShortcut(ui.Cmd, ui.KeyR, func() { m.load(h) })

	ui.Column(c).Fill().Children(func() {
		kit.PageHeader(c, kit.Tr(h.Lang(), "快照仓库"), kit.Tr(h.Lang(), "{0} 个仓库", len(m.rows)), func() {
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Placeholder(kit.Tr(h.Lang(), "筛选仓库…")).Width(200)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.load(h)
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "新建仓库…")).Clicked() {
				m.showNew, m.newName, m.newErr = true, "", ""
				m.newJSON = settingsTemplate(m.newType)
				m.newFocus = true
			}
		})

		if m.actionErr != "" {
			ui.Text(c, m.actionErr).FontSize(kit.SizeCaption(t)).TextColor(t.Danger).Padding(0, t.Space(5), t.Space(1), t.Space(5))
		}
		if m.actionMsg != "" {
			ui.Text(c, m.actionMsg).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted).Padding(0, t.Space(5), t.Space(1), t.Space(5))
		}
		if client == nil {
			ui.Text(c, kit.Tr(h.Lang(), "未连接集群")).TextColor(t.TextMuted).Padding(0, t.Space(4))
			return
		}
		if kit.StateText(c, h.Lang(), m.loading && !m.loadedOnce, m.err, len(m.visibleRows()) == 0, kit.Tr(h.Lang(), "还没有仓库")) {
			m.dialogs(h, c)
			return
		}

		rows := m.visibleRows()
		if m.selected >= len(rows) {
			m.selected = -1
		}
		listState := ui.ListState{Selected: &m.selected, Key: func(i int) any { return rows[i].Name }, Columns: m.prefs.Frame(h, "repositories")}
		ui.Table(c, &listState, repoCols(h.Lang()), len(rows), func(row, col int) {
			r := rows[row]
			switch col {
			case 0:
				ui.Text(c, r.Name).SingleLine()
			case 1:
				ui.Text(c, r.Type)
			case 2:
				ui.Text(c, settingsSummary(r.Settings)).TextColor(t.TextMuted).SingleLine()
			case 3:
				ui.Row(c).Gap(t.Space(1.5)).Children(func() {
					if ui.Button(c, kit.Tr(h.Lang(), "验证")).Clicked() {
						m.actionErr, m.actionMsg = "", ""
						m.verify(h, r.Name)
					}
					if ui.Button(c, kit.Tr(h.Lang(), "删除…")).Clicked() {
						m.deleteName, m.deleteConfirm = r.Name, true
					}
				})
			}
		}).Grow(1).Padding(0, t.Space(3), 0, t.Space(3))
		// ui.Table 在拖拽时原地改写 ListState.Columns（列变动没有专门
		// 回调），每帧把改完的布局交回 Sync 比较并持久化。
		m.prefs.Sync(h, "repositories", listState.Columns)

		m.dialogs(h, c)
	})
}

func (m *Module) dialogs(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()

	ui.Modal(c, &m.showNew, func() {
		ui.Text(c, kit.Tr(h.Lang(), "新建仓库")).Bold()
		ui.Form(c, func() {
			ui.Field(c, kit.Tr(h.Lang(), "仓库名称"), func() {
				input := ui.TextInput(c, &m.newName)
				if m.newFocus {
					input.AutoFocus()
					m.newFocus = false
				}
			})
			ui.Field(c, kit.Tr(h.Lang(), "类型"), func() {
				if ui.Select(c, &m.newType, []string{"fs", "s3", "url"}).Changed() {
					m.newJSON = settingsTemplate(m.newType)
				}
			})
			ui.Field(c, kit.Tr(h.Lang(), "设置 JSON"), func() {
				ui.TextInput(c, &m.newJSON)
			}).Description(kit.Tr(h.Lang(), "仓库 settings，如 {0}", strings.Join(strings.Fields(settingsTemplate(m.newType)), " ")))
		})
		if m.newErr != "" {
			ui.Text(c, m.newErr).FontSize(kit.SizeCaption(t)).TextColor(t.Danger)
		}
		ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
			if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
				m.showNew = false
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "创建")).Clicked() {
				m.create(h)
			}
		})
	})

	if m.deleteConfirm {
		name := m.deleteName
		ui.Modal(c, &m.deleteConfirm, func() {
			ui.Text(c, kit.Tr(h.Lang(), "删除仓库")).Bold()
			ui.Text(c, kit.Tr(h.Lang(), "删除后快照会失效，确定删 {0}？", name)).TextColor(t.TextMuted)
			ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
					m.deleteName, m.deleteConfirm = "", false
				}
				if kit.DangerButton(c, kit.Tr(h.Lang(), "删除")).Clicked() {
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
	run(h, "repositories", func() { m.loading = true }, func(ctx context.Context) ([]RepoRow, error) {
		var entries map[string]repoEntry
		if _, err := client.CatRepositoriesInto(ctx, nil, &entries); err != nil {
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
		m.newErr = kit.Tr(h.Lang(), "名称不能为空")
		return
	}
	var settings map[string]any
	if err := json.Unmarshal([]byte(m.newJSON), &settings); err != nil {
		m.newErr = kit.Tr(h.Lang(), "设置 JSON 无效：{0}", err.Error())
		return
	}
	client := h.ES()
	if client == nil {
		return
	}
	body := map[string]any{"type": m.newType, "settings": settings}
	m.newErr = ""
	run(h, "repositories-action", func() {}, func(ctx context.Context) (*es.Response, error) {
		return client.SnapshotCreateRepository(ctx, m.newName, body)
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
	run(h, "repositories-action", func() { m.loading = true }, func(ctx context.Context) (*es.Response, error) {
		return client.Request(ctx, "GET", "_snapshot/"+name+"/_verify", nil)
	}, func(_ *es.Response, err error) {
		m.loading = false
		if err == nil {
			m.actionMsg = kit.Tr(h.Lang(), "{0} 验证通过", name)
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

	run(h, "repositories-action", func() { m.actionErr, m.actionMsg = "", "" }, func(ctx context.Context) (*es.Response, error) {
		return client.SnapshotDeleteRepository(ctx, name)
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
// result on the main thread. key 是取消槽位（同 key 新加载取消旧的）。
func run[T any](h kit.ModuleHost, key string, mark func(), fn func(ctx context.Context) (T, error), apply func(T, error)) {
	h.Load(key, mark, func(ctx context.Context) func() {
		v, err := fn(ctx)
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
