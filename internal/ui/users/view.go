package users

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
	showEnabled := es.SupportsUserEnabledInBody(es.MajorVersion(h.Version()))
	showDeprecated := es.SupportsSecurityDeprecatedMetadata(h.Version())
	// ⌘R 刷新，系统习惯。
	c.OnShortcut(ui.Cmd, ui.KeyR, func() { m.load(h) })

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(t.Space(4), t.Space(5), t.Space(2), t.Space(5)).Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, kit.Tr(h.Lang(), "用户")).FontSize(kit.SizeTitle(t)).Bold()
				ui.Text(c, kit.Tr(h.Lang(), "{0} 个用户", len(m.rows))).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Placeholder(kit.Tr(h.Lang(), "筛选用户…")).Width(200)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.load(h)
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "新建用户")).Clicked() {
				m.form = userForm{Enabled: true}
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
		if kit.StateText(c, h.Lang(), m.loading, m.err, len(m.rows) == 0, kit.Tr(h.Lang(), "还没有用户")) {
			return
		}

		rows := m.visibleRows()
		if m.selected >= len(rows) {
			m.selected = -1
		}

		cols := []ui.TableColumn{
			{Title: kit.Tr(h.Lang(), "用户名"), Sortable: true},
			{Title: kit.Tr(h.Lang(), "角色")},
			{Title: kit.Tr(h.Lang(), "全名")},
			{Title: kit.Tr(h.Lang(), "邮箱")},
		}
		kinds := []cellCol{colUsername, colRoles, colFullName, colEmail}
		if showEnabled {
			cols = append(cols, ui.TableColumn{Title: kit.Tr(h.Lang(), "启用"), Width: 56})
			kinds = append(kinds, colEnabled)
		}
		cols = append(cols, ui.TableColumn{Title: kit.Tr(h.Lang(), "系统保留"), Width: 56})
		kinds = append(kinds, colReserved)
		if showDeprecated {
			cols = append(cols, ui.TableColumn{Title: kit.Tr(h.Lang(), "已弃用"), Width: 56})
			kinds = append(kinds, colDeprecated)
		}

		state := ui.ListState{Selected: &m.selected, Sort: &m.sort, Columns: m.prefs.Frame(h, "users")}
		table := ui.Table(c, &state, cols, len(rows), func(row, col int) {
			userCell(c, m, rows[row], kinds[col], h.Lang())
		}).Grow(1).Padding(0, t.Space(3), 0, t.Space(3))
		// ui.Table 在拖拽时原地改写 ListState.Columns（列变动没有专门
		// 回调），每帧把改完的布局交回 Sync 比较并持久化。
		m.prefs.Sync(h, "users", state.Columns)
		if table.Submitted() && m.selected >= 0 && m.selected < len(rows) {
			u := rows[m.selected]
			if u.Reserved {
				m.openPasswordDialog(u)
			} else {
				m.openEditDialog(u)
			}
		}
	})
}

// cellCol names the logical cells of the users table.
type cellCol int

const (
	colUsername cellCol = iota
	colRoles
	colFullName
	colEmail
	colEnabled
	colReserved
	colDeprecated
)

func userCell(c *ui.Context, m *Module, u userRow, col cellCol, lang string) {
	t := c.Theme()
	switch col {
	case colUsername:
		ui.Text(c, u.Username).SingleLine().ContextMenu(func(mn *ui.Menu) {
			if !u.Reserved {
				if mn.Item(kit.Tr(lang, "编辑…")).Chosen() {
					m.openEditDialog(u)
				}
			}
			if mn.Item(kit.Tr(lang, "修改密码…")).Chosen() {
				m.openPasswordDialog(u)
			}
			mn.Separator()
			if !u.Reserved {
				if mn.Item(kit.Tr(lang, "删除…")).Chosen() {
					m.confirmName, m.confirmOpen = u.Username, true
				}
			}
		})
	case colRoles:
		ui.Text(c, strings.Join(u.Roles, ", ")).SingleLine().TextColor(t.TextMuted)
	case colFullName:
		ui.Text(c, u.FullName).SingleLine()
	case colEmail:
		ui.Text(c, u.Email).SingleLine()
	case colEnabled:
		ui.Text(c, kit.Tr(lang, boolText(u.Enabled)))
	case colReserved:
		if u.Reserved {
			ui.Text(c, kit.Tr(lang, "是")).TextColor(t.TextMuted)
		}
	case colDeprecated:
		txt := ui.Text(c, kit.Tr(lang, boolText(u.Deprecated)))
		if u.DeprecatedReason != "" {
			txt.Tooltip(u.DeprecatedReason)
		}
	}
}

func boolText(b bool) string {
	if b {
		return "是"
	}
	return "否"
}
func (m *Module) openEditDialog(u userRow) {
	m.form = userForm{
		Username: u.Username,
		Roles:    strings.Join(u.Roles, ", "),
		FullName: u.FullName,
		Email:    u.Email,
		Enabled:  u.Enabled,
	}
	m.dialog, m.dialogOpen, m.submitTried = dialogEdit, true, false
}

func (m *Module) openPasswordDialog(u userRow) {
	m.form = userForm{Username: u.Username}
	m.dialog, m.dialogOpen, m.submitTried = dialogPassword, true, false
}

// unsupportedNotice is the whole page on OpenSearch, whose security API is
// not the Elasticsearch native one elasticvue's user management needs.
func unsupportedNotice(c *ui.Context, lang string) {
	t := c.Theme()
	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(t.Space(2)).Children(func() {
		ui.Text(c, kit.Tr(lang, "用户")).FontSize(kit.SizeTitle(t)).Bold()
		ui.Text(c, kit.Tr(lang, "OpenSearch 不支持原生用户/角色管理")).TextColor(t.TextMuted)
	})
}

// buildDialogs builds the open modal ahead of the page so its texts come
// first when tests look for them.
func (m *Module) buildDialogs(h kit.ModuleHost, c *ui.Context) {
	if m.dialogOpen {
		switch m.dialog {
		case dialogNew:
			m.userDialog(h, c, false)
		case dialogEdit:
			m.userDialog(h, c, true)
		case dialogPassword:
			m.passwordDialog(h, c)
		}
	}
	if m.confirmOpen && m.confirmName != "" {
		m.confirmDialog(h, c)
	}
}

// userDialog is the create/edit form; edit makes the username read-only and
// the password optional.
func (m *Module) userDialog(h kit.ModuleHost, c *ui.Context, edit bool) {
	f := &m.form
	title, submit := kit.Tr(h.Lang(), "新建用户"), kit.Tr(h.Lang(), "创建")
	if edit {
		title, submit = kit.Tr(h.Lang(), "编辑用户"), kit.Tr(h.Lang(), "保存")
	}
	ui.Modal(c, &m.dialogOpen, func() {
		ui.Column(c).Width(440).Padding(c.Theme().Space(5)).Gap(c.Theme().Space(3)).Children(func() {
			ui.Text(c, title).FontSize(kit.SizeTitle(c.Theme())).Bold()

			var nameErr, passErr, rolesErr string
			if m.submitTried {
				if strings.TrimSpace(f.Username) == "" {
					nameErr = kit.Tr(h.Lang(), "用户名不能为空")
				}
				if f.Password == "" && !edit {
					passErr = kit.Tr(h.Lang(), "密码不能为空")
				}
				if len(splitList(f.Roles)) == 0 {
					rolesErr = kit.Tr(h.Lang(), "至少选一个角色")
				}
			}
			valid := strings.TrimSpace(f.Username) != "" && (edit || f.Password != "") && len(splitList(f.Roles)) > 0

			ui.Field(c, kit.Tr(h.Lang(), "用户名"), func() {
				input := ui.TextInput(c, &f.Username)
				if !edit {
					input.AutoFocus()
				}
				input.ReadOnly(edit)
			}).Error(nameErr)
			ui.Field(c, kit.Tr(h.Lang(), "密码"), func() {
				input := ui.TextInput(c, &f.Password).Password()
				if edit {
					input.AutoFocus()
				}
			}).Description(passwordHint(edit, h.Lang())).Error(passErr)
			ui.Field(c, kit.Tr(h.Lang(), "角色"), func() {
				ui.TextInput(c, &f.Roles)
			}).Description(kit.Tr(h.Lang(), "逗号分隔，如 kibana_admin")).Error(rolesErr)
			ui.Field(c, kit.Tr(h.Lang(), "全名"), func() {
				ui.TextInput(c, &f.FullName)
			})
			ui.Field(c, kit.Tr(h.Lang(), "邮箱"), func() {
				ui.TextInput(c, &f.Email)
			})
			if es.SupportsUserEnabledInBody(es.MajorVersion(h.Version())) {
				ui.Field(c, kit.Tr(h.Lang(), "启用"), func() {
					ui.Checkbox(c, &f.Enabled, kit.Tr(h.Lang(), "启用该用户"))
				})
			}

			ui.Row(c).Gap(c.Theme().Space(2)).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
					m.dialogOpen = false
				}
				if ui.PrimaryButton(c, submit).Disabled(m.busy).Clicked() {
					m.submitTried = true
					if valid {
						if edit {
							m.updateUser(h)
						} else {
							m.createUser(h)
						}
					}
				}
			})
		})
	})
}

func passwordHint(edit bool, lang string) string {
	if edit {
		return kit.Tr(lang, "留空则不修改密码")
	}
	return ""
}

// passwordDialog changes one user's password via _password.
func (m *Module) passwordDialog(h kit.ModuleHost, c *ui.Context) {
	f := &m.form
	ui.Modal(c, &m.dialogOpen, func() {
		ui.Column(c).Width(440).Padding(c.Theme().Space(5)).Gap(c.Theme().Space(3)).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "修改密码")).FontSize(kit.SizeTitle(c.Theme())).Bold()
			ui.Text(c, f.Username).TextColor(c.Theme().TextMuted)

			var passErr string
			if m.submitTried && f.Password == "" {
				passErr = kit.Tr(h.Lang(), "密码不能为空")
			}
			ui.Field(c, kit.Tr(h.Lang(), "新密码"), func() {
				ui.TextInput(c, &f.Password).Password().AutoFocus()
			}).Error(passErr)

			ui.Row(c).Gap(c.Theme().Space(2)).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
					m.dialogOpen = false
				}
				if ui.PrimaryButton(c, kit.Tr(h.Lang(), "修改")).Disabled(m.busy).Clicked() {
					m.submitTried = true
					if f.Password != "" {
						m.changePassword(h)
					}
				}
			})
		})
	})
}

// confirmDialog asks before a delete.
func (m *Module) confirmDialog(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	ui.Modal(c, &m.confirmOpen, func() {
		ui.Column(c).Width(380).Padding(t.Space(5)).Gap(t.Space(3)).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "删除用户")).FontSize(kit.SizeTitle(t)).Bold()
			ui.Text(c, kit.Tr(h.Lang(), "删了就恢复不了，确定删 {0}？", m.confirmName)).TextColor(t.TextMuted)
			ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
					m.confirmOpen = false
				}
				if kit.DangerButton(c, kit.Tr(h.Lang(), "删除")).Disabled(m.busy).Clicked() {
					m.deleteUser(h)
				}
			})
		})
	})
}
