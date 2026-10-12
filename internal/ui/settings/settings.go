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

var settingsCols = []ui.TableColumn{
	{Title: "区块", Width: 110},
	{Title: "键", Sortable: true},
	{Title: "值"},
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	client := h.ES()
	if client == nil {
		t := c.Theme()
		ui.Column(c).Fill().Padding(16).Children(func() {
			ui.Text(c, "集群设置").FontSize(20).Bold()
			ui.Text(c, "未连接集群").TextColor(t.TextMuted)
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
				ui.Text(c, "集群设置").FontSize(19).Bold()
				ui.Text(c, fmt.Sprintf("%d 项设置", len(m.rows))).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			if ui.Button(c, "刷新").Clicked() {
				m.load(h)
			}
			if m.selectedRow >= 0 && m.selectedRow < len(m.rows) {
				if ui.Button(c, "编辑…").Clicked() {
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
			ui.Text(c, m.actionErr).FontSize(12).TextColor(ui.Hex("#d64545")).Padding(0, 20, 4, 20)
		}
		if kit.StateText(c, m.loading, m.err, len(m.rows) == 0, "没有集群设置") {
			return
		}

		if m.selectedRow >= len(m.rows) {
			m.selectedRow = -1
		}
		state := ui.ListState{Selected: &m.selectedRow}
		table := ui.Table(c, &state, settingsCols, len(m.rows), func(row, col int) {
			r := m.rows[row]
			switch col {
			case 0:
				ui.Text(c, map[bool]string{true: "transient", false: "persistent"}[r.section == "transient"]).
					FontSize(12).TextColor(sectionColor(c, r.section))
			case 1:
				ui.Text(c, r.key).SingleLine()
			case 2:
				ui.Text(c, r.value).FontSize(12).SingleLine().TextColor(t.TextMuted)
			}
		}).Grow(1).Padding(0, 12, 0, 12)
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

func sectionColor(c *ui.Context, section string) ui.Color {
	if section == "transient" {
		return ui.Hex("#d9a406")
	}
	return ui.Hex("#2f9e63")
}

func (m *Module) editDialog(h kit.ModuleHost, c *ui.Context) {
	if !m.editing {
		return
	}
	t := c.Theme()
	ui.Modal(c, &m.editing, func() {
		ui.Text(c, "修改集群设置").Bold()
		if !m.confirming {
			ui.Column(c).Gap(8).Children(func() {
				ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, m.editKey).Bold()
					ui.Text(c, "当前值: "+m.editOld).FontSize(12).TextColor(t.TextMuted)
				})
				ui.TextArea(c, &m.editValue).Label("新值（JSON 或纯文本）").Lines(2, 8)
				ui.Checkbox(c, &m.transient, "写入 transient（重启后失效）")
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					if ui.Button(c, "取消").Clicked() {
						m.editing = false
					}
					if ui.Button(c, "应用…").Clicked() {
						m.confirming = true
					}
				})
			})
			return
		}
		// 二次确认：改集群设置是危险操作。
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, "将把 "+sectionName(m.transient)+" 设置 "+m.editKey+" 改为 "+m.editValue).TextColor(t.TextMuted)
			ui.Text(c, "集群设置变更会影响整个集群，确定继续吗？").
				FontSize(12).TextColor(ui.Hex("#d64545"))
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "返回").Clicked() {
					m.confirming = false
				}
				if ui.Button(c, "确认修改").Clicked() {
					key, value, transient := m.editKey, m.editValue, m.transient
					m.editing = false
					m.apply(h, key, value, transient)
				}
			})
		})
	})
}

func sectionName(transient bool) string {
	if transient {
		return "transient"
	}
	return "persistent"
}

// apply PUTs the single edited key, then reloads.
func (m *Module) apply(h kit.ModuleHost, key, value string, transient bool) {
	client := h.ES()
	if client == nil {
		return
	}
	parsed, ok := parseValue(value)
	if !ok {
		m.actionErr = "新值不是合法 JSON"
		return
	}
	section := "persistent"
	if transient {
		section = "transient"
	}
	body := map[string]any{section: map[string]any{key: parsed}}
	h.Load(
		func() { m.actionErr = "" },
		func() func() {
			_, err := client.ClusterPutSettings(context.Background(), body)
			return func() {
				if err != nil {
					m.actionErr = "修改失败: " + kit.ErrMessage(err)
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
	h.Load(
		func() { m.loading = true },
		func() func() {
			resp, err := client.ClusterGetSettings(context.Background())
			var rows []settingRow
			if err == nil {
				rows, err = flattenSettings(resp.Body)
			}
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
func flattenSettings(body []byte) ([]settingRow, error) {
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(body, &doc); err != nil {
		return nil, err
	}
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
