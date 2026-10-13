// Package settings builds the「集群设置」module page: _cluster/settings
// with persistent/transient sections and single-key editing.
package settings

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/ui/kit"
)

// Module holds the 设置 module's state.
type Module struct {
	loadedOnce bool
	loading    bool
	err        error

	rows        []settingRow
	selectedRow int // 选中的行号，-1 为无

	actionErr string

	// 编辑对话框
	editing    bool
	editKey    string
	editOld    string
	editValue  string
	transient  bool
	confirming bool // 危险操作二次确认
}

// New makes the module's state.
func New() *Module { return &Module{selectedRow: -1} }

// settingRow is one flattened cluster setting.
type settingRow struct {
	section string // persistent 或 transient
	key     string // 展平后的点分键
	value   string
}

func settingsCols(lang string) []ui.TableColumn {
	return []ui.TableColumn{
		{Title: kit.Tr(lang, "分组"), Width: 110},
		{Title: kit.Tr(lang, "键"), Sortable: true},
		{Title: kit.Tr(lang, "值")},
	}
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	client := h.ES()
	if client == nil {
		ui.Column(c).Fill().Children(func() {
			kit.PageHeader(c, kit.Tr(h.Lang(), "集群设置"), kit.Tr(h.Lang(), "未连接集群"), nil)
		})
		return
	}
	if !m.loadedOnce && !m.loading {
		m.load(h)
	}
	t := c.Theme()
	// ⌘R 刷新，系统习惯。
	c.OnShortcut(ui.Cmd, ui.KeyR, func() { m.load(h) })

	ui.Column(c).Fill().Children(func() {
		kit.PageHeader(c, kit.Tr(h.Lang(), "集群设置"), kit.Tr(h.Lang(), "{0} 项设置", len(m.rows)), func() {
			ui.Spacer(c)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.load(h)
			}
			if m.selectedRow >= 0 && m.selectedRow < len(m.rows) {
				if ui.Button(c, kit.Tr(h.Lang(), "编辑…")).Clicked() {
					r := m.rows[m.selectedRow]
					m.editing, m.confirming = true, false
					m.editKey = r.key
					m.editOld = r.value
					m.editValue = r.value
					m.transient = r.section == "transient"
				}
			}
		})

		if m.actionErr != "" {
			ui.Text(c, m.actionErr).FontSize(kit.SizeCaption(t)).TextColor(t.Danger).Padding(0, t.Space(5), t.Space(1), t.Space(5))
		}
		if kit.StateText(c, h.Lang(), m.loading, m.err, len(m.rows) == 0, kit.Tr(h.Lang(), "还没有集群设置")) {
			return
		}

		if m.selectedRow >= len(m.rows) {
			m.selectedRow = -1
		}
		state := ui.ListState{Selected: &m.selectedRow}
		table := ui.Table(c, &state, settingsCols(h.Lang()), len(m.rows), func(row, col int) {
			r := m.rows[row]
			switch col {
			case 0:
				ui.Text(c, sectionLabel(h.Lang(), r.section == "transient")).
					TextColor(kit.LevelColor(c.Theme(), sectionLevel(r.section)))
			case 1:
				ui.Text(c, r.key).SingleLine()
			case 2:
				ui.Text(c, r.value).SingleLine().TextColor(t.TextMuted)
			}
		}).Grow(1).Padding(0, t.Space(3), 0, t.Space(3))
		if table.Submitted() && m.selectedRow >= 0 && m.selectedRow < len(m.rows) {
			r := m.rows[m.selectedRow]
			m.editing, m.confirming = true, false
			m.editKey = r.key
			m.editOld = r.value
			m.editValue = r.value
			m.transient = r.section == "transient"
		}
	})

	m.editDialog(h, c)
}

func sectionLevel(section string) kit.Level {
	if section == "transient" {
		return kit.LevelWarning
	}
	return kit.LevelSuccess
}

func (m *Module) editDialog(h kit.ModuleHost, c *ui.Context) {
	if !m.editing {
		return
	}
	t := c.Theme()
	ui.Modal(c, &m.editing, func() {
		ui.Text(c, kit.Tr(h.Lang(), "修改集群设置")).Bold()
		if !m.confirming {
			ui.Column(c).Gap(t.Space(2)).Children(func() {
				ui.Row(c).Gap(t.Space(2)).Children(func() {
					ui.Text(c, m.editKey).Bold()
					ui.Text(c, kit.Tr(h.Lang(), "当前值：{0}", m.editOld)).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
				})
				ui.TextArea(c, &m.editValue).Label(kit.Tr(h.Lang(), "新值（JSON 或纯文本）")).Lines(2, 8)
				ui.Checkbox(c, &m.transient, kit.Tr(h.Lang(), "重启后失效"))
				ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
					if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
						m.editing = false
					}
					if ui.Button(c, kit.Tr(h.Lang(), "应用…")).Clicked() {
						m.confirming = true
					}
				})
			})
			return
		}
		// 二次确认：改集群设置是危险操作。
		ui.Column(c).Gap(t.Space(2)).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "会把 {0} 的 {1} 改成 {2}", sectionLabel(h.Lang(), m.transient), m.editKey, m.editValue)).TextColor(t.TextMuted)
			ui.Text(c, kit.Tr(h.Lang(), "会影响整个集群，确定改吗？")).
				FontSize(kit.SizeCaption(t)).TextColor(t.Danger)
			ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "返回")).Clicked() {
					m.confirming = false
				}
				if kit.DangerButton(c, kit.Tr(h.Lang(), "确认修改")).Clicked() {
					key, value, transient := m.editKey, m.editValue, m.transient
					m.editing = false
					m.apply(h, key, value, transient)
				}
			})
		})
	})
}

func sectionLabel(lang string, transient bool) string {
	if transient {
		return kit.Tr(lang, "临时")
	}
	return kit.Tr(lang, "永久")
}

// apply PUTs the single edited key, then reloads.
func (m *Module) apply(h kit.ModuleHost, key, value string, transient bool) {
	client := h.ES()
	if client == nil {
		return
	}
	parsed, ok := parseValue(value)
	if !ok {
		m.actionErr = kit.Tr(h.Lang(), "新值不是合法 JSON")
		return
	}
	section := "persistent"
	if transient {
		section = "transient"
	}
	body := map[string]any{section: map[string]any{key: parsed}}
	h.Load("settings-put",
		func() { m.actionErr = "" },
		func(ctx context.Context) func() {
			_, err := client.ClusterPutSettings(ctx, body)
			return func() {
				if err != nil {
					m.actionErr = kit.Tr(h.Lang(), "修改失败：{0}", kit.ErrMessage(err))
				}
				m.load(h)
			}
		},
	)
}

// parseValue keeps JSON values typed, plain strings as strings.
func parseValue(v string) (any, bool) {
	trimmed := strings.TrimSpace(v)
	var parsed any
	if err := json.Unmarshal([]byte(trimmed), &parsed); err == nil {
		return parsed, true
	}
	return trimmed, true
}

func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load("settings",
		func() { m.loading = true },
		func(ctx context.Context) func() {
			var doc map[string]json.RawMessage
			var rows []settingRow
			err := func() error {
				if _, err := client.ClusterGetSettingsInto(ctx, &doc); err != nil {
					return err
				}
				var ferr error
				rows, ferr = flattenSettings(doc)
				return ferr
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

// flattenSettings flattens the persistent/transient sections into dotted keys.
func flattenSettings(doc map[string]json.RawMessage) ([]settingRow, error) {
	var rows []settingRow
	for _, section := range []string{"persistent", "transient"} {
		raw, ok := doc[section]
		if !ok {
			continue
		}
		values := map[string]any{}
		if err := json.Unmarshal(raw, &values); err != nil {
			return nil, err
		}
		flatten(section, "", values, &rows)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].section != rows[j].section {
			return rows[i].section == "persistent"
		}
		return rows[i].key < rows[j].key
	})
	return rows, nil
}

func flatten(section, prefix string, values map[string]any, rows *[]settingRow) {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		key := k
		if prefix != "" {
			key = prefix + "." + k
		}
		if nested, ok := values[k].(map[string]any); ok && len(nested) > 0 {
			flatten(section, key, nested, rows)
			continue
		}
		*rows = append(*rows, settingRow{section: section, key: key, value: formatValue(values[k])})
	}
}

func formatValue(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		if v == float64(int64(v)) {
			return fmt.Sprintf("%d", int64(v))
		}
		return fmt.Sprintf("%g", v)
	default:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	}
}
