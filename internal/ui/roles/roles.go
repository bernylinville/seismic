// Package roles builds the「角色」module page: the cluster's _security/role
// list with JSON inspection, create, edit and delete, mirroring elasticvue's
// cluster roles page.
package roles

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

// defaultIndicesJSON is what a new role's indices field starts from, as
// elasticvue's DEFAULT_INDICES_JSON.
const defaultIndicesJSON = `[
  {
    "names": ["*"],
    "privileges": ["read"]
  }
]`

// roleRow is one _security/role entry, mapped the way elasticvue's
// mapElasticsearchRoles maps it.
type roleRow struct {
	Name             string
	Description      string
	Cluster          []string
	IndicesOverview  string
	Reserved         bool
	Deprecated       bool
	DeprecatedReason string
	JSON             string // pretty-printed raw body, for 查看 JSON
	raw              map[string]any
}

// esRole is the part of the raw role object the table reads.
type esRole struct {
	Description string          `json:"description"`
	Cluster     []string        `json:"cluster"`
	Indices     []esIndexEntry  `json:"indices"`
	Metadata    *esRoleMetadata `json:"metadata"`
}

type esIndexEntry struct {
	Names      []string `json:"names"`
	Privileges []string `json:"privileges"`
}

// esRoleMetadata carries the reserved/deprecated flags elasticvue reads.
type esRoleMetadata struct {
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
)

// roleForm is what the create/edit dialog edits; Cluster and RunAs are
// comma-separated, IndicesJSON is the indices privilege array as JSON.
type roleForm struct {
	Name        string
	Description string
	Cluster     string
	IndicesJSON string
	RunAs       string
}

// Module holds the 角色 module's state.
type Module struct {
	rows       []roleRow
	filter     string
	loading    bool
	err        error
	loadedOnce bool

	selected int
	sort     ui.SortOrder
	prefs    kit.ColumnPrefs

	dialogOpen  bool
	dialog      dialogKind
	form        roleForm
	submitTried bool
	busy        bool

	confirmOpen bool
	confirmName string

	detailName string // 查看 JSON 的角色，空为无

	actionMsg string // 最近一次写操作的结果提示
	actionErr bool
}

// New makes the module's state.
func New() *Module { return &Module{} }

// visibleRows filters the rows on the filter and sorts them by name.
func (m *Module) visibleRows() []roleRow {
	q := strings.ToLower(m.filter)
	out := make([]roleRow, 0, len(m.rows))
	for _, r := range m.rows {
		if q != "" && !strings.Contains(strings.ToLower(r.Name+"\n"+r.Description+"\n"+strings.Join(r.Cluster, "\n")), q) {
			continue
		}
		out = append(out, r)
	}
	slices.SortStableFunc(out, func(a, b roleRow) int {
		c := strings.Compare(a.Name, b.Name)
		if m.sort.Descending {
			return -c
		}
		return c
	})
	return out
}

// load fetches the role list off-thread and applies it on the main thread.
func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load(func() { m.loading = true }, func() func() {
		rows, err := fetchRoles(client)
		return func() {
			m.loading = false
			m.loadedOnce = true
			m.err = err
			if err == nil {
				m.rows = rows
				if m.detailName != "" && !slices.ContainsFunc(rows, func(r roleRow) bool { return r.Name == m.detailName }) {
					m.detailName = ""
				}
			}
		}
	})
}

func fetchRoles(client *es.Client) ([]roleRow, error) {
	resp, err := client.SecurityRoles(context.Background())
	if err != nil {
		return nil, err
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(resp.Body, &raw); err != nil {
		return nil, err
	}
	rows := make([]roleRow, 0, len(raw))
	for name, body := range raw {
		var role esRole
		if err := json.Unmarshal(body, &role); err != nil {
			return nil, err
		}
		var rawMap map[string]any
		if err := json.Unmarshal(body, &rawMap); err != nil {
			return nil, err
		}
		pretty, err := json.MarshalIndent(rawMap, "", "  ")
		if err != nil {
			return nil, err
		}
		r := roleRow{
			Name:            name,
			Description:     role.Description,
			Cluster:         append([]string(nil), role.Cluster...),
			IndicesOverview: indicesOverview(role.Indices),
			JSON:            string(pretty),
			raw:             rawMap,
		}
		slices.Sort(r.Cluster)
		if role.Metadata != nil {
			r.Reserved = role.Metadata.Reserved
			r.Deprecated = role.Metadata.Deprecated
			r.DeprecatedReason = role.Metadata.DeprecatedReason
		}
		rows = append(rows, r)
	}
	slices.SortFunc(rows, func(a, b roleRow) int { return strings.Compare(a.Name, b.Name) })
	return rows, nil
}

// indicesOverview summarizes a role's indices privileges as
// “names: privileges” per entry, joined by “；”.
func indicesOverview(entries []esIndexEntry) string {
	parts := make([]string, 0, len(entries))
	for _, e := range entries {
		names := strings.Join(e.Names, ", ")
		privs := strings.Join(e.Privileges, ", ")
		switch {
		case names != "" && privs != "":
			parts = append(parts, names+": "+privs)
		case names != "":
			parts = append(parts, names)
		default:
			parts = append(parts, strings.Join(e.Privileges, ", "))
		}
	}
	return strings.Join(parts, "；")
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

// roleFormFromRow fills the edit form from a role's raw body, as
// elasticvue's roleFormFromClusterRole does.
func roleFormFromRow(r roleRow) roleForm {
	f := roleForm{
		Name:        r.Name,
		Description: r.Description,
		Cluster:     strings.Join(r.Cluster, ", "),
		RunAs:       strings.Join(stringList(r.raw["run_as"]), ", "),
	}
	if indices, err := json.MarshalIndent(r.raw["indices"], "", "  "); err == nil {
		f.IndicesJSON = string(indices)
	}
	return f
}

func stringList(v any) []string {
	list, _ := v.([]any)
	out := make([]string, 0, len(list))
	for _, e := range list {
		if s, ok := e.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

// validIndicesJSON reports whether s parses as a JSON array, as
// elasticvue's isValidIndicesJson.
func validIndicesJSON(s string) bool {
	var v any
	if err := json.Unmarshal([]byte(s), &v); err != nil {
		return false
	}
	_, ok := v.([]any)
	return ok
}

// sanitizeIndices strips allow_restricted_indices from each entry on
// versions without it, as elasticvue's sanitizeIndices does.
func sanitizeIndices(indices []any, allowRestricted bool) []any {
	if allowRestricted {
		return indices
	}
	for i, e := range indices {
		if m, ok := e.(map[string]any); ok {
			delete(m, "allow_restricted_indices")
			indices[i] = m
		}
	}
	return indices
}

// buildRoleBody builds the PUT body as elasticvue's buildRoleBody does:
// editing starts from the existing raw body, then the version gates apply
// (applications from 6.4, allow_restricted_indices from 6.7, description
// from 8.15) and metadata keys starting with “_” are stripped.
func buildRoleBody(f roleForm, existing map[string]any, version string) (map[string]any, error) {
	body := map[string]any{}
	maps.Copy(body, existing)
	delete(body, "name")
	delete(body, "transient_metadata")

	var indices []any
	if err := json.Unmarshal([]byte(f.IndicesJSON), &indices); err != nil {
		return nil, err
	}
	body["indices"] = sanitizeIndices(indices, es.SupportsAllowRestrictedIndices(version))
	body["cluster"] = splitList(f.Cluster)
	body["run_as"] = splitList(f.RunAs)
	if es.SupportsRoleDescription(version) {
		body["description"] = strings.TrimSpace(f.Description)
	} else {
		delete(body, "description")
	}
	if !es.SupportsRoleApplications(version) {
		delete(body, "applications")
	}
	if md, ok := body["metadata"].(map[string]any); ok {
		for k := range md {
			if strings.HasPrefix(k, "_") {
				delete(md, k)
			}
		}
		if len(md) == 0 {
			delete(body, "metadata")
		}
	}
	return body, nil
}

// report records a finished write's outcome; okKey renders success,
// failKey failure, both with the role name as {0}.
func (m *Module) report(h kit.ModuleHost, okKey, failKey, name string, err error) {
	m.busy = false
	if err != nil {
		m.actionMsg, m.actionErr = kit.Tr(h.Lang(), failKey, name, kit.ErrMessage(err)), true
		return
	}
	m.actionMsg, m.actionErr = kit.Tr(h.Lang(), okKey, name), false
}

// putRole PUTs the form off-thread, then reloads the list; when editing it
// starts from the role's existing raw body.
func (m *Module) putRole(h kit.ModuleHost, name, okKey, failKey string) {
	client := h.ES()
	if client == nil {
		return
	}
	var existing map[string]any
	if m.dialog == dialogEdit {
		for _, r := range m.rows {
			if r.Name == name {
				existing = r.raw
			}
		}
	}
	body, err := buildRoleBody(m.form, existing, h.Version())
	if err != nil {
		m.actionMsg, m.actionErr = kit.Tr(h.Lang(), failKey, name, err.Error()), true
		return
	}
	h.Load(func() { m.busy = true }, func() func() {
		_, err := client.SecurityRolePut(context.Background(), name, body)
		return func() {
			m.report(h, okKey, failKey, name, err)
			if err == nil {
				m.dialogOpen = false
				m.load(h)
			}
		}
	})
}

// createRole PUTs a new role.
func (m *Module) createRole(h kit.ModuleHost) {
	name := strings.TrimSpace(m.form.Name)
	m.putRole(h, name, "已创建角色 {0}", "创建角色 {0}失败: {1}")
}

// updateRole PUTs an edited role.
func (m *Module) updateRole(h kit.ModuleHost) {
	name := strings.TrimSpace(m.form.Name)
	m.putRole(h, name, "已更新角色 {0}", "更新角色 {0}失败: {1}")
}

// deleteRole DELETEs a role off-thread, then reloads the list.
func (m *Module) deleteRole(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	name := m.confirmName
	h.Load(func() { m.busy = true; m.confirmOpen = false }, func() func() {
		_, err := client.SecurityRoleDelete(context.Background(), name)
		return func() {
			m.report(h, "已删除角色 {0}", "删除角色 {0}失败: {1}", name, err)
			if err == nil {
				m.confirmName = ""
				m.load(h)
			}
		}
	})
}
