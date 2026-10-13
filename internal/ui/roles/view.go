package roles

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	m.buildDialogs(h, c)

	if !es.SupportsNativeSecurityAPI(h.Version(), h.Distribution()) {
		unsupportedNotice(c, h.Lang())
		return
	}
	if !m.loadedOnce && !m.loading && h.ES() != nil {
		m.load(h)
	}
	t := c.Theme()
	showDescription := es.SupportsRoleDescription(h.Version())
	// ⌘R 刷新，系统习惯。
	c.OnShortcut(ui.Cmd, ui.KeyR, func() { m.load(h) })

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(t.Space(4), t.Space(5), t.Space(2), t.Space(5)).Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, kit.Tr(h.Lang(), "角色")).FontSize(kit.SizeTitle(t)).Bold()
				ui.Text(c, kit.Tr(h.Lang(), "{0} 个角色", len(m.rows))).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Placeholder(kit.Tr(h.Lang(), "筛选角色…")).Width(200)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.load(h)
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "新建角色")).Clicked() {
				m.form = roleForm{IndicesJSON: defaultIndicesJSON}
				m.dialog, m.dialogOpen, m.submitTried = dialogNew, true, false
			}
		})

		if m.actionMsg != "" {
			color := t.Text
			if m.actionErr {
				color = t.Danger
			}
			ui.Text(c, m.actionMsg).FontSize(kit.SizeCaption(t)).TextColor(color).Padding(0, t.Space(5), t.Space(1), t.Space(5))
		}
		if kit.StateText(c, h.Lang(), m.loading, m.err, len(m.rows) == 0, kit.Tr(h.Lang(), "还没有角色")) {
			return
		}

		rows := m.visibleRows()
		if m.selected >= len(rows) {
			m.selected = -1
		}

		cols := []ui.TableColumn{{Title: kit.Tr(h.Lang(), "名称"), Sortable: true}}
		kinds := []cellCol{colName}
		if showDescription {
			cols = append(cols, ui.TableColumn{Title: kit.Tr(h.Lang(), "描述")})
			kinds = append(kinds, colDescription)
		}
		cols = append(cols,
			ui.TableColumn{Title: kit.Tr(h.Lang(), "索引权限")},
			ui.TableColumn{Title: kit.Tr(h.Lang(), "集群权限")},
			ui.TableColumn{Title: kit.Tr(h.Lang(), "系统保留"), Width: 56},
		)
		kinds = append(kinds, colIndices, colCluster, colReserved)

		state := ui.ListState{Selected: &m.selected, Sort: &m.sort, Columns: m.prefs.Frame(h, "roles")}
		table := ui.Table(c, &state, cols, len(rows), func(row, col int) {
			roleCell(c, m, rows[row], kinds[col], h.Lang())
		}).Grow(1).Padding(0, t.Space(3), 0, t.Space(3))
		// ui.Table 在拖拽时原地改写 ListState.Columns（列变动没有专门
		// 回调），每帧把改完的布局交回 Sync 比较并持久化。
		m.prefs.Sync(h, "roles", state.Columns)
		if table.Submitted() && m.selected >= 0 && m.selected < len(rows) {
			m.detailName = rows[m.selected].Name
		}

		if m.detailName != "" {
			m.detailView(c, rows, h.Lang())
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

func roleCell(c *ui.Context, m *Module, r roleRow, col cellCol, lang string) {
	t := c.Theme()
	switch col {
	case colName:
		ui.Text(c, r.Name).SingleLine().ContextMenu(func(menu *ui.Menu) {
			if menu.Item(kit.Tr(lang, "查看 JSON")).Chosen() {
				m.detailName = r.Name
			}
			menu.Separator()
			if !r.Reserved {
				if menu.Item(kit.Tr(lang, "编辑…")).Chosen() {
					m.form = roleFormFromRow(r)
					m.dialog, m.dialogOpen, m.submitTried = dialogEdit, true, false
				}
				if menu.Item(kit.Tr(lang, "删除…")).Chosen() {
					m.confirmName, m.confirmOpen = r.Name, true
				}
			}
		})
	case colDescription:
		ui.Text(c, r.Description).SingleLine().TextColor(t.TextMuted)
	case colIndices:
		ui.Text(c, r.IndicesOverview).SingleLine().TextColor(t.TextMuted)
	case colCluster:
		ui.Text(c, strings.Join(r.Cluster, ", ")).SingleLine()
	case colReserved:
		if r.Reserved {
			ui.Text(c, kit.Tr(lang, "是")).TextColor(t.TextMuted)
		}
	}
}

// detailView shows the chosen role's raw JSON below the table.
func (m *Module) detailView(c *ui.Context, rows []roleRow, lang string) {
	t := c.Theme()
	ui.Column(c).Height(t.Space(55)).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
		Background(t.Surface).Padding(t.Space(3), t.Space(4), t.Space(3), t.Space(4)).Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m.detailName).Bold()
			ui.Text(c, kit.Tr(lang, "· JSON")).TextColor(t.TextMuted).FontSize(kit.SizeCaption(t))
			ui.Spacer(c)
			if ui.Button(c, kit.Tr(lang, "关闭")).Clicked() {
				m.detailName = ""
			}
		})
		for _, r := range rows {
			if r.Name == m.detailName {
				ui.Text(c, r.JSON).Font("monospace").FontSize(kit.SizeMono(t)).Grow(1)
			}
		}
	})
}

// unsupportedNotice is the whole page on OpenSearch, whose security API is
// not the Elasticsearch native one elasticvue's role management needs.
func unsupportedNotice(c *ui.Context, lang string) {
	t := c.Theme()
	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(t.Space(2)).Children(func() {
		ui.Text(c, kit.Tr(lang, "角色")).FontSize(kit.SizeTitle(t)).Bold()
		ui.Text(c, kit.Tr(lang, "OpenSearch 不支持原生用户/角色管理")).TextColor(t.TextMuted)
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
	title, submit := kit.Tr(h.Lang(), "新建角色"), kit.Tr(h.Lang(), "创建")
	if edit {
		title, submit = kit.Tr(h.Lang(), "编辑角色"), kit.Tr(h.Lang(), "保存")
	}
	version := h.Version()
	ui.Modal(c, &m.dialogOpen, func() {
		ui.Column(c).Width(520).Padding(c.Theme().Space(5)).Gap(c.Theme().Space(3)).Children(func() {
			ui.Text(c, title).FontSize(kit.SizeTitle(c.Theme())).Bold()

			var nameErr, indicesErr string
			if m.submitTried {
				if strings.TrimSpace(f.Name) == "" {
					nameErr = kit.Tr(h.Lang(), "名称不能为空")
				}
				if !validIndicesJSON(f.IndicesJSON) {
					indicesErr = kit.Tr(h.Lang(), "必须是 JSON 数组")
				}
			}
			valid := strings.TrimSpace(f.Name) != "" && validIndicesJSON(f.IndicesJSON)

			ui.Field(c, kit.Tr(h.Lang(), "名称"), func() {
				input := ui.TextInput(c, &f.Name)
				if !edit {
					input.AutoFocus()
				}
				input.ReadOnly(edit)
			}).Error(nameErr)
			if es.SupportsRoleDescription(version) {
				ui.Field(c, kit.Tr(h.Lang(), "描述"), func() {
					ui.TextInput(c, &f.Description)
				})
			}
			ui.Field(c, kit.Tr(h.Lang(), "集群权限"), func() {
				ui.TextInput(c, &f.Cluster)
			}).Description(kit.Tr(h.Lang(), "逗号分隔，如 all, monitor"))
			ui.Field(c, kit.Tr(h.Lang(), "索引权限"), func() {
				ui.TextArea(c, &f.IndicesJSON).Height(140)
			}).Description(indicesHint(version, h.Lang())).Error(indicesErr)
			ui.Field(c, "run_as", func() {
				ui.TextInput(c, &f.RunAs)
			}).Description(kit.Tr(h.Lang(), "代替的用户，逗号分隔"))

			ui.Row(c).Gap(c.Theme().Space(2)).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
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
func indicesHint(version, lang string) string {
	hint := kit.Tr(lang, "JSON 数组，每项含 names 与 privileges")
	if es.SupportsAllowRestrictedIndices(version) {
		hint += kit.Tr(lang, "，可用 allow_restricted_indices")
	}
	return hint
}

// confirmDialog asks before a delete.
func (m *Module) confirmDialog(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	ui.Modal(c, &m.confirmOpen, func() {
		ui.Column(c).Width(380).Padding(t.Space(5)).Gap(t.Space(3)).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "删除角色")).FontSize(kit.SizeTitle(t)).Bold()
			ui.Text(c, kit.Tr(h.Lang(), "删了就恢复不了，确定删 {0}？", m.confirmName)).TextColor(t.TextMuted)
			ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
					m.confirmOpen = false
				}
				if kit.DangerButton(c, kit.Tr(h.Lang(), "删除")).Disabled(m.busy).Clicked() {
					m.deleteRole(h)
				}
			})
		})
	})
}
