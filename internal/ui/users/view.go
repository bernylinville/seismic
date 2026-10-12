package users

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
	showEnabled := es.SupportsUserEnabledInBody(es.MajorVersion(h.Version()))
	showDeprecated := es.SupportsSecurityDeprecatedMetadata(h.Version())

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 20, 8, 20).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, "用户").FontSize(19).Bold()
				ui.Text(c, fmt.Sprintf("%d 个用户", len(m.rows))).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &m.filter).Placeholder("筛选用户…").Width(200)
			if ui.Button(c, "刷新").Clicked() {
				m.load(h)
			}
			if ui.PrimaryButton(c, "新建用户").Clicked() {
				m.form = userForm{Enabled: true}
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
		if kit.StateText(c, m.loading, m.err, len(m.rows) == 0, "没有用户") {
			return
		}

		rows := m.visibleRows()
		if m.selected >= len(rows) {
			m.selected = -1
		}

		cols := []ui.TableColumn{
			{Title: "用户名", Sortable: true},
			{Title: "角色"},
			{Title: "全名"},
			{Title: "邮箱"},
		}
		kinds := []cellCol{colUsername, colRoles, colFullName, colEmail}
		if showEnabled {
			cols = append(cols, ui.TableColumn{Title: "启用", Width: 56})
			kinds = append(kinds, colEnabled)
		}
		cols = append(cols, ui.TableColumn{Title: "保留", Width: 56})
		kinds = append(kinds, colReserved)
		if showDeprecated {
			cols = append(cols, ui.TableColumn{Title: "弃用", Width: 56})
			kinds = append(kinds, colDeprecated)
		}

		state := ui.ListState{Selected: &m.selected, Sort: &m.sort}
		table := ui.Table(c, &state, cols, len(rows), func(row, col int) {
			userCell(c, m, rows[row], kinds[col])
		}).Grow(1).Padding(0, 12, 0, 12)
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

func userCell(c *ui.Context, m *Module, u userRow, col cellCol) {
	t := c.Theme()
	switch col {
	case colUsername:
		ui.Text(c, u.Username).SingleLine().ContextMenu(func(mn *ui.Menu) {
			if !u.Reserved {
				if mn.Item("编辑…").Chosen() {
					m.openEditDialog(u)
				}
			}
			if mn.Item("修改密码…").Chosen() {
				m.openPasswordDialog(u)
			}
			mn.Separator()
			if !u.Reserved {
				if mn.Item("删除…").Chosen() {
					m.confirmName, m.confirmOpen = u.Username, true
				}
			}
		})
	case colRoles:
		ui.Text(c, strings.Join(u.Roles, ", ")).FontSize(12).SingleLine().TextColor(t.TextMuted)
	case colFullName:
		ui.Text(c, u.FullName).FontSize(12).SingleLine()
	case colEmail:
		ui.Text(c, u.Email).FontSize(12).SingleLine()
	case colEnabled:
		ui.Text(c, boolText(u.Enabled)).FontSize(12)
	case colReserved:
		if u.Reserved {
			ui.Text(c, "是").FontSize(12).TextColor(t.TextMuted)
		}
	case colDeprecated:
		txt := ui.Text(c, boolText(u.Deprecated)).FontSize(12)
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
func unsupportedNotice(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().AlignItems(ui.Center).Justify(ui.Center).Gap(8).Children(func() {
		ui.Text(c, "用户").FontSize(19).Bold()
		ui.Text(c, "OpenSearch 不支持原生用户/角色管理").TextColor(t.TextMuted)
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
	title, submit := "新建用户", "创建"
	if edit {
		title, submit = "编辑用户", "保存"
	}
	ui.Modal(c, &m.dialogOpen, func() {
		ui.Column(c).Width(440).Padding(20).Gap(12).Children(func() {
			ui.Text(c, title).FontSize(16).Bold()

			var nameErr, passErr, rolesErr string
			if m.submitTried {
				if strings.TrimSpace(f.Username) == "" {
					nameErr = "用户名不能为空"
				}
				if f.Password == "" && !edit {
					passErr = "密码不能为空"
				}
				if len(splitList(f.Roles)) == 0 {
					rolesErr = "至少一个角色"
				}
			}
			valid := strings.TrimSpace(f.Username) != "" && (edit || f.Password != "") && len(splitList(f.Roles)) > 0

			ui.Field(c, "用户名", func() {
				input := ui.TextInput(c, &f.Username)
				if !edit {
					input.AutoFocus()
				}
				input.ReadOnly(edit)
			}).Error(nameErr)
			ui.Field(c, "密码", func() {
				input := ui.TextInput(c, &f.Password).Password()
				if edit {
					input.AutoFocus()
				}
			}).Description(passwordHint(edit)).Error(passErr)
			ui.Field(c, "角色", func() {
				ui.TextInput(c, &f.Roles)
			}).Description("逗号分隔，如 kibana_admin").Error(rolesErr)
			ui.Field(c, "全名", func() {
				ui.TextInput(c, &f.FullName)
			})
			ui.Field(c, "邮箱", func() {
				ui.TextInput(c, &f.Email)
			})
			if es.SupportsUserEnabledInBody(es.MajorVersion(h.Version())) {
				ui.Field(c, "启用", func() {
					ui.Checkbox(c, &f.Enabled, "启用该用户")
				})
			}

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
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

func passwordHint(edit bool) string {
	if edit {
		return "留空则不修改密码"
	}
	return ""
}

// passwordDialog changes one user's password via _password.
func (m *Module) passwordDialog(h kit.ModuleHost, c *ui.Context) {
	f := &m.form
	ui.Modal(c, &m.dialogOpen, func() {
		ui.Column(c).Width(440).Padding(20).Gap(12).Children(func() {
			ui.Text(c, "修改密码").FontSize(16).Bold()
			ui.Text(c, f.Username).TextColor(c.Theme().TextMuted)

			var passErr string
			if m.submitTried && f.Password == "" {
				passErr = "密码不能为空"
			}
			ui.Field(c, "新密码", func() {
				ui.TextInput(c, &f.Password).Password().AutoFocus()
			}).Error(passErr)

			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					m.dialogOpen = false
				}
				if ui.PrimaryButton(c, "修改").Disabled(m.busy).Clicked() {
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
		ui.Column(c).Width(380).Padding(20).Gap(12).Children(func() {
			ui.Text(c, "删除用户").FontSize(16).Bold()
			ui.Text(c, "删除用户 "+m.confirmName+"？此操作不可撤销。").TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					m.confirmOpen = false
				}
				if ui.Button(c, "删除").Background(ui.Hex("#d64545")).TextColor(ui.Hex("#ffffff")).Disabled(m.busy).Clicked() {
					m.deleteUser(h)
				}
			})
		})
	})
}
