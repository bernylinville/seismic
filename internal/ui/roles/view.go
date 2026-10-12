package roles

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	m.buildDialogs(h, c)

	if !es.SupportsNativeSecurityAPI(h.Version(), h.Distribution()) {
		unsupportedNotice(c)
		return
	}
	if !m.loadedOnce && !m.loading && h.ES() != nil {
		m.load(h)
	}
	t := c.Theme()
	showDescription := es.SupportsRoleDescription(h.Version())

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 20, 8, 20).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, "角色").FontSize(19).Bold()
				ui.Text(c, fmt.Sprintf("%d 个角色", len(m.rows))).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Placeholder("筛选角色…").Width(200)
			if ui.Button(c, "刷新").Clicked() {
				m.load(h)
			}
			if ui.PrimaryButton(c, "新建角色").Clicked() {
				m.form = roleForm{IndicesJSON: defaultIndicesJSON}
				m.dialog, m.dialogOpen, m.submitTried = dialogNew, true, false
			}
		})

		if m.actionMsg != "" {
			color := t.Text
			if m.actionErr {
				color = ui.Hex("#d64545")
			}
			ui.Text(c, m.actionMsg).FontSize(12).TextColor(color).Padding(0, 20, 4, 20)
		}
		if kit.StateText(c, m.loading, m.err, len(m.rows) == 0, "没有角色") {
			return
		}

		rows := m.visibleRows()
		if m.selected >= len(rows) {
			m.selected = -1
		}

		cols := []ui.TableColumn{{Title: "名称", Sortable: true}}
		kinds := []cellCol{colName}
		if showDescription {
			cols = append(cols, ui.TableColumn{Title: "描述"})
			kinds = append(kinds, colDescription)
		}
		cols = append(cols,
			ui.TableColumn{Title: "索引权限"},
			ui.TableColumn{Title: "集群权限"},
			ui.TableColumn{Title: "保留", Width: 56},
		)
		kinds = append(kinds, colIndices, colCluster, colReserved)

		state := ui.ListState{Selected: &m.selected, Sort: &m.sort}
		table := ui.Table(c, &state, cols, len(rows), func(row, col int) {
			roleCell(c, m, rows[row], kinds[col])
		}).Grow(1).Padding(0, 12, 0, 12)
		if table.Submitted() && m.selected >= 0 && m.selected < len(rows) {
			m.detailName = rows[m.selected].Name
		}

		if m.detailName != "" {
			m.detailView(c, rows)
		}
	})
}

// cellCol names the logical cells of the roles table.
type cellCol int

const (
	colName cellCol = iota
	colDescription
	colIndices
	colCluster
	colReserved
)

func roleCell(c *ui.Context, m *Module, r roleRow, col cellCol) {
	t := c.Theme()
	switch col {
	case colName:
		ui.Text(c, r.Name).SingleLine().ContextMenu(func(menu *ui.Menu) {
			if menu.Item("查看 JSON").Chosen() {
				m.detailName = r.Name
			}
			menu.Separator()
			if !r.Reserved {
				if menu.Item("编辑…").Chosen() {
					m.form = roleFormFromRow(r)
					m.dialog, m.dialogOpen, m.submitTried = dialogEdit, true, false
				}
				if menu.Item("删除…").Chosen() {
					m.confirmName, m.confirmOpen = r.Name, true
				}
			}
		})
	case colDescription:
		ui.Text(c, r.Description).FontSize(12).SingleLine().TextColor(t.TextMuted)
	case colIndices:
		ui.Text(c, r.IndicesOverview).FontSize(12).SingleLine().TextColor(t.TextMuted)
	case colCluster:
		ui.Text(c, strings.Join(r.Cluster, ", ")).FontSize(12).SingleLine()
	case colReserved:
		if r.Reserved {
			ui.Text(c, "是").FontSize(12).TextColor(t.TextMuted)
		}
	}
}

// detailView shows the chosen role's raw JSON below the table.
func (m *Module) detailView(c *ui.Context, rows []roleRow) {
	t := c.Theme()
	ui.Column(c).Height(220).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
		Background(t.Surface).Padding(12, 16, 12, 16).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m.detailName).Bold()
			ui.Text(c, "· JSON").TextColor(t.TextMuted).FontSize(12)
			ui.Spacer(c)
			if ui.Button(c, "关闭").Clicked() {
				m.detailName = ""
			}
		})
		for _, r := range rows {
			if r.Name == m.detailName {
				ui.Text(c, r.JSON).FontSize(12).Grow(1)
			}
		}
	})
}

// unsupportedNotice is the whole page on OpenSearch, whose security API is
// not the Elasticsearch native one elasticvue's role management needs.
func unsupportedNotice(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
		ui.Text(c, "角色").FontSize(19).Bold()
		ui.Text(c, "OpenSearch 不支持原生用户/角色管理").TextColor(t.TextMuted)
	})
}

// buildDialogs builds the open modal ahead of the page so its texts come
// first when tests look for them.
func (m *Module) buildDialogs(h kit.ModuleHost, c *ui.Context) {
	if m.dialogOpen {
		switch m.dialog {
		case dialogNew:
			m.roleDialog(h, c, false)
		case dialogEdit:
			m.roleDialog(h, c, true)
		}
	}
	if m.confirmOpen && m.confirmName != "" {
		m.confirmDialog(h, c)
	}
}

// roleDialog is the create/edit form; edit makes the name read-only, and
// the description field only exists from 8.15 on.
func (m *Module) roleDialog(h kit.ModuleHost, c *ui.Context, edit bool) {
	f := &m.form
	title, submit := "新建角色", "创建"
	if edit {
		title, submit = "编辑角色", "保存"
	}
	version := h.Version()
	ui.Modal(c, &m.dialogOpen, func() {
		ui.Column(c).Width(520).Padding(20).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(16).Bold()

			var nameErr, indicesErr string
			if m.submitTried {
				if strings.TrimSpace(f.Name) == "" {
					nameErr = "名称不能为空"
				}
				if !validIndicesJSON(f.IndicesJSON) {
					indicesErr = "必须是 JSON 数组"
				}
			}
			valid := strings.TrimSpace(f.Name) != "" && validIndicesJSON(f.IndicesJSON)

			ui.Field(c, "名称", func() {
				input := ui.TextInput(c, &f.Name)
				if !edit {
					input.AutoFocus()
				}
				input.ReadOnly(edit)
			}).Error(nameErr)
			if es.SupportsRoleDescription(version) {
				ui.Field(c, "描述", func() {
					ui.TextInput(c, &f.Description)
				})
			}
			ui.Field(c, "集群权限", func() {
				ui.TextInput(c, &f.Cluster)
			}).Description("逗号分隔，如 all, monitor")
			ui.Field(c, "索引权限", func() {
				ui.TextArea(c, &f.IndicesJSON).Height(140)
			}).Description(indicesHint(version)).Error(indicesErr)
			ui.Field(c, "run_as", func() {
				ui.TextInput(c, &f.RunAs)
			}).Description("逗号分隔的代替用户")

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					m.dialogOpen = false
				}
				if ui.PrimaryButton(c, submit).Disabled(m.busy).Clicked() {
					m.submitTried = true
					if valid {
						if edit {
							m.updateRole(h)
						} else {
							m.createRole(h)
						}
					}
				}
			})
		})
	})
}

// indicesHint explains the indices JSON, mentioning allow_restricted_indices
// only on versions that have it (6.7+).
func indicesHint(version string) string {
	hint := "JSON 数组，每项含 names 与 privileges"
	if es.SupportsAllowRestrictedIndices(version) {
		hint += "，可用 allow_restricted_indices"
	}
	return hint
}

// confirmDialog asks before a delete.
func (m *Module) confirmDialog(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	ui.Modal(c, &m.confirmOpen, func() {
		ui.Column(c).Width(380).Padding(20).Gap(12).Children(func() {
			ui.Text(c, "删除角色").FontSize(16).Bold()
			ui.Text(c, "删除角色 "+m.confirmName+"？此操作不可撤销。").TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					m.confirmOpen = false
				}
				if ui.Button(c, "删除").Background(ui.Hex("#d64545")).TextColor(ui.Hex("#ffffff")).Disabled(m.busy).Clicked() {
					m.deleteRole(h)
				}
			})
		})
	})
}
