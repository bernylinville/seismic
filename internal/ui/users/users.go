// Package users builds the「用户」module page: the cluster's _security/user
// list with create, edit, password change and delete, mirroring elasticvue's
// cluster users page.
package users

import (
	"context"
	"encoding/json"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// userRow is one _security/user entry, mapped the way elasticvue's
// mapElasticsearchUsers maps it.
type userRow struct {
	Username         string
	Roles            []string
	FullName         string
	Email            string
	Enabled          bool
	Reserved         bool
	Deprecated       bool
	DeprecatedReason string
}

// esUser is the raw user object found under its username key.
type esUser struct {
	Roles    []string        `json:"roles"`
	FullName string          `json:"full_name"`
	Email    string          `json:"email"`
	Enabled  *bool           `json:"enabled"`
	Metadata *esUserMetadata `json:"metadata"`
}

// esUserMetadata carries the reserved/deprecated flags elasticvue reads.
type esUserMetadata struct {
	Reserved         bool   `json:"_reserved"`
	Deprecated       bool   `json:"_deprecated"`
	DeprecatedReason string `json:"_deprecated_reason"`
}

// dialogKind says which modal, if any, is open.
type dialogKind int

const (
	dialogClosed dialogKind = iota
	dialogNew
	dialogEdit
	dialogPassword
)

// userForm is what the create/edit dialogs edit; Roles is comma-separated.
type userForm struct {
	Username string
	Password string
	Roles    string
	FullName string
	Email    string
	Enabled  bool
}

// Module holds the 用户 module's state.
type Module struct {
	rows       []userRow
	filter     string
	loading    bool
	err        error
	loadedOnce bool

	selected int
	sort     ui.SortOrder
	prefs    kit.ColumnPrefs

	dialogOpen  bool
	dialog      dialogKind
	form        userForm
	submitTried bool
	busy        bool

	confirmOpen bool
	confirmName string

	actionMsg string // 最近一次写操作的结果提示
	actionErr bool
}

// New makes the module's state.
func New() *Module { return &Module{} }

// visibleRows filters the rows on the filter and sorts them by username.
func (m *Module) visibleRows() []userRow {
	q := strings.ToLower(m.filter)
	out := make([]userRow, 0, len(m.rows))
	for _, r := range m.rows {
		if q != "" && !strings.Contains(strings.ToLower(r.Username+"\n"+r.FullName+"\n"+r.Email+"\n"+strings.Join(r.Roles, "\n")), q) {
			continue
		}
		out = append(out, r)
	}
	slices.SortStableFunc(out, func(a, b userRow) int {
		c := strings.Compare(a.Username, b.Username)
		if m.sort.Descending {
			return -c
		}
		return c
	})
	return out
}

// load fetches the user list off-thread and applies it on the main thread.
func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load(func() { m.loading = true }, func() func() {
		rows, err := fetchUsers(client)
		return func() {
			m.loading = false
			m.loadedOnce = true
			m.err = err
			if err == nil {
				m.rows = rows
			}
		}
	})
}

func fetchUsers(client *es.Client) ([]userRow, error) {
	resp, err := client.SecurityUsers(context.Background())
	if err != nil {
		return nil, err
	}
	var raw map[string]esUser
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return nil, err
	}
	rows := make([]userRow, 0, len(raw))
	for name, u := range raw {
		roles := append([]string(nil), u.Roles...)
		slices.Sort(roles)
		enabled := u.Enabled == nil || *u.Enabled
		r := userRow{Username: name, Roles: roles, FullName: u.FullName, Email: u.Email, Enabled: enabled}
		if u.Metadata != nil {
			r.Reserved = u.Metadata.Reserved
			r.Deprecated = u.Metadata.Deprecated
			r.DeprecatedReason = u.Metadata.DeprecatedReason
		}
		rows = append(rows, r)
	}
	slices.SortFunc(rows, func(a, b userRow) int { return strings.Compare(a.Username, b.Username) })
	return rows, nil
}

// splitList splits a comma-separated form field into clean values.
func splitList(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// userBody builds the PUT body as elasticvue's buildUserBody does: enabled
// only from 6 on, password only when asked for.
func userBody(f userForm, version string, includePassword bool) map[string]any {
	body := map[string]any{
		"roles":     splitList(f.Roles),
		"full_name": strings.TrimSpace(f.FullName),
		"email":     strings.TrimSpace(f.Email),
	}
	if es.SupportsUserEnabledInBody(es.MajorVersion(version)) {
		body["enabled"] = f.Enabled
	}
	if includePassword {
		body["password"] = f.Password
	}
	return body
}

// report records a finished write's outcome; okKey renders success,
// failKey failure, both with the username as {0}.
func (m *Module) report(h kit.ModuleHost, okKey, failKey, name string, err error) {
	m.busy = false
	if err != nil {
		m.actionMsg, m.actionErr = kit.Tr(h.Lang(), failKey, name, kit.ErrMessage(err)), true
		return
	}
	m.actionMsg, m.actionErr = kit.Tr(h.Lang(), okKey, name), false
}

// createUser PUTs a new user off-thread, then reloads the list.
func (m *Module) createUser(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	name := strings.TrimSpace(m.form.Username)
	body := userBody(m.form, h.Version(), true)
	h.Load(func() { m.busy = true }, func() func() {
		_, err := client.SecurityUserPut(context.Background(), name, body)
		return func() {
			m.report(h, "已创建用户 {0}", "创建用户 {0}失败: {1}", name, err)
			if err == nil {
				m.dialogOpen = false
				m.load(h)
			}
		}
	})
}

// updateUser PUTs an edited user off-thread (password only when filled in),
// then reloads the list.
func (m *Module) updateUser(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	name := strings.TrimSpace(m.form.Username)
	body := userBody(m.form, h.Version(), m.form.Password != "")
	h.Load(func() { m.busy = true }, func() func() {
		_, err := client.SecurityUserPut(context.Background(), name, body)
		return func() {
			m.report(h, "已更新用户 {0}", "更新用户 {0}失败: {1}", name, err)
			if err == nil {
				m.dialogOpen = false
				m.load(h)
			}
		}
	})
}

// changePassword PUTs a new password off-thread, then reloads the list.
func (m *Module) changePassword(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	name := strings.TrimSpace(m.form.Username)
	body := map[string]any{"password": m.form.Password}
	h.Load(func() { m.busy = true }, func() func() {
		_, err := client.SecurityUserPutPassword(context.Background(), name, body)
		return func() {
			m.report(h, "已修改 {0} 的密码", "修改 {0} 的密码失败: {1}", name, err)
			if err == nil {
				m.dialogOpen = false
				m.load(h)
			}
		}
	})
}

// deleteUser DELETEs a user off-thread, then reloads the list.
func (m *Module) deleteUser(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	name := m.confirmName
	h.Load(func() { m.busy = true; m.confirmOpen = false }, func() func() {
		_, err := client.SecurityUserDelete(context.Background(), name)
		return func() {
			m.report(h, "已删除用户 {0}", "删除用户 {0}失败: {1}", name, err)
			if err == nil {
				m.confirmName = ""
				m.load(h)
			}
		}
	})
}
