package roles

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
)

// fakeHost is a kit.ModuleHost whose Load applies synchronously, so tests
// see request results in the same frame.
type fakeHost struct {
	client       *es.Client
	version      string
	distribution string
}

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return "test-cluster" }
func (f *fakeHost) Version() string      { return f.version }
func (f *fakeHost) Distribution() string { return f.distribution }
func (f *fakeHost) BuildFlavor() string  { return "" }
func (f *fakeHost) DB() *db.Store        { return nil }
func (f *fakeHost) Load(mark func(), fn func() func()) {
	mark()
	fn()()
}

// fakeES records every request and answers the security endpoints.
type fakeES struct {
	mu       sync.Mutex
	requests []request
	roles    string
}

type request struct {
	method, path, body string
}

func (f *fakeES) all() []request {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]request(nil), f.requests...)
}

func (f *fakeES) handler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	f.mu.Lock()
	f.requests = append(f.requests, request{r.Method, r.URL.Path, string(body)})
	f.mu.Unlock()
	if r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/role") {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, f.roles)
		return
	}
	io.WriteString(w, `{}`)
}

const testRoles = `{
  "superuser": {"cluster":["all"],"metadata":{"_reserved":true}},
  "reader": {
    "description":"只读访问",
    "cluster":["monitor"],
    "indices":[{"names":["logs-*"],"privileges":["read"],"allow_restricted_indices":false}],
    "applications":[{"application":"kibana","privileges":["all"]}],
    "run_as":["alice"],
    "metadata":{"_reserved":false}
  },
  "writer": {"cluster":[],"indices":[{"names":["metrics-*"],"privileges":["write"]}]}
}`

// newHarness wires a module to a fake cluster and a headless view.
func newHarness(t *testing.T, version, distribution string) (*Module, *fakeES, *ui.Tester) {
	t.Helper()
	fake := &fakeES{roles: testRoles}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(srv.Close)
	client, err := es.New(srv.URL, es.Auth{Type: es.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	client.Version = version
	host := &fakeHost{client: client, version: version, distribution: distribution}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 960, 640)
	return m, fake, tt
}

func (f *fakeES) find(t *testing.T, method, path string) request {
	t.Helper()
	for _, r := range f.all() {
		if r.method == method && r.path == path {
			return r
		}
	}
	t.Fatalf("no %s %s among %v", method, path, f.all())
	return request{}
}

func TestRolesListCurrentVersion(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	for _, want := range []string{"reader", "superuser", "writer", "logs-*: read", "metrics-*: write", "保留", "3 个角色"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts: %q", want, tt.Texts())
		}
	}
	if tt.HasText("描述") {
		t.Errorf("description column should be hidden before 8.15; texts: %q", tt.Texts())
	}
	fake.find(t, http.MethodGet, "/_security/role")
}

func TestRolesListDescriptionColumn(t *testing.T) {
	_, _, tt := newHarness(t, "8.15.0", "elasticsearch")

	if !tt.HasText("描述") || !tt.HasText("只读访问") {
		t.Errorf("description column should show from 8.15 on; texts: %q", tt.Texts())
	}
}

func TestRolesOldVersion(t *testing.T) {
	_, fake, tt := newHarness(t, "6.5.0", "elasticsearch")

	fake.find(t, http.MethodGet, "/_xpack/security/role")

	if err := tt.Click("新建角色"); err != nil {
		t.Fatal(err)
	}
	if tt.HasText("描述") {
		t.Errorf("description field should be hidden before 8.15; texts: %q", tt.Texts())
	}
	if tt.HasText("allow_restricted_indices") {
		t.Errorf("allow_restricted_indices hint should be hidden before 6.7; texts: %q", tt.Texts())
	}
	if !tt.HasText("索引权限") {
		t.Errorf("missing indices field; texts: %q", tt.Texts())
	}
}

func TestCreateDialogHintNewVersion(t *testing.T) {
	_, _, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.Click("新建角色"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("allow_restricted_indices") {
		t.Errorf("hint should mention allow_restricted_indices from 6.7 on; texts: %q", tt.Texts())
	}
	if tt.HasText("描述") {
		t.Errorf("description field should be hidden before 8.15; texts: %q", tt.Texts())
	}
}

func TestViewRoleJSON(t *testing.T) {
	_, _, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.RightClick("reader"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("查看 JSON"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"· JSON", `"privileges"`, `"logs-*"`} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in JSON pane; texts: %q", want, tt.Texts())
		}
	}
}

func TestEditRoleStripsOldVersionFields(t *testing.T) {
	_, fake, tt := newHarness(t, "6.5.0", "elasticsearch")

	if err := tt.RightClick("reader"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("编辑…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("编辑角色") {
		t.Fatalf("edit dialog did not open; texts: %q", tt.Texts())
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}

	req := fake.find(t, http.MethodPut, "/_xpack/security/role/reader")
	var body map[string]any
	if err := json.Unmarshal([]byte(req.body), &body); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	if _, ok := body["applications"]; !ok {
		t.Errorf("applications must be kept from 6.4 on: %v", body)
	}
	indices := body["indices"].([]any)
	entry := indices[0].(map[string]any)
	if _, ok := entry["allow_restricted_indices"]; ok {
		t.Errorf("allow_restricted_indices must be stripped before 6.7: %v", entry)
	}
	if entry["names"] != nil && !strings.Contains(req.body, "logs-*") {
		t.Errorf("index names lost: %v", req.body)
	}
	if !tt.HasText("已更新角色 reader") {
		t.Errorf("missing success message; texts: %q", tt.Texts())
	}
}

func TestEditRoleDropsApplicationsBefore64(t *testing.T) {
	_, fake, tt := newHarness(t, "6.3.0", "elasticsearch")

	if err := tt.RightClick("reader"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("编辑…"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}

	req := fake.find(t, http.MethodPut, "/_xpack/security/role/reader")
	var body map[string]any
	if err := json.Unmarshal([]byte(req.body), &body); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	if _, ok := body["applications"]; ok {
		t.Errorf("applications must be dropped before 6.4: %v", body)
	}
	if _, ok := body["description"]; ok {
		t.Errorf("description must be dropped before 8.15: %v", body)
	}
}

func TestEditRoleKeepsNewVersionFields(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.RightClick("reader"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("编辑…"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}

	req := fake.find(t, http.MethodPut, "/_security/role/reader")
	var body map[string]any
	if err := json.Unmarshal([]byte(req.body), &body); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	if _, ok := body["applications"]; !ok {
		t.Errorf("applications must be kept from 6.4 on: %v", body)
	}
	entry := body["indices"].([]any)[0].(map[string]any)
	if v, ok := entry["allow_restricted_indices"]; !ok || v != false {
		t.Errorf("allow_restricted_indices must be kept from 6.7 on: %v", entry)
	}
	if _, ok := body["description"]; ok {
		t.Errorf("description must be dropped before 8.15: %v", body)
	}
}

func TestCreateRole(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.Click("新建角色"); err != nil {
		t.Fatal(err)
	}
	tt.Type("reporter")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}

	req := fake.find(t, http.MethodPut, "/_security/role/reporter")
	var body map[string]any
	if err := json.Unmarshal([]byte(req.body), &body); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	entry := body["indices"].([]any)[0].(map[string]any)
	if names, ok := entry["names"].([]any); !ok || len(names) != 1 || names[0] != "*" {
		t.Errorf("default indices names = %v", entry["names"])
	}
	if privs, ok := entry["privileges"].([]any); !ok || len(privs) != 1 || privs[0] != "read" {
		t.Errorf("default indices privileges = %v", entry["privileges"])
	}
	if _, ok := body["description"]; ok {
		t.Errorf("description must be absent before 8.15: %v", body)
	}
	if !tt.HasText("已创建角色 reporter") {
		t.Errorf("missing success message; texts: %q", tt.Texts())
	}
}

func TestCreateRoleValidation(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.Click("新建角色"); err != nil {
		t.Fatal(err)
	}
	// Clear the prefilled indices JSON to make the form invalid too.
	tt.Key(0, ui.KeyTab)
	tt.Key(0, ui.KeyTab)
	tt.Type("{invalid")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("名称不能为空") || !tt.HasText("必须是 JSON 数组") {
		t.Errorf("missing validation errors; texts: %q", tt.Texts())
	}
	for _, r := range fake.all() {
		if r.method == http.MethodPut {
			t.Errorf("invalid form must not be submitted: %v", r)
		}
	}
}

func TestReservedRoleIsReadOnly(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.RightClick("superuser"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	for _, item := range menu {
		if item == "编辑…" || item == "删除…" {
			t.Errorf("reserved role offers %q; menu: %q", item, menu)
		}
	}
	tt.CloseMenu()
	if reqs := fake.all(); len(reqs) != 1 {
		t.Errorf("unexpected requests: %v", reqs)
	}
}

func TestDeleteRole(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.RightClick("writer"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("删除角色 writer？") {
		t.Fatalf("confirm dialog did not open; texts: %q", tt.Texts())
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}

	fake.find(t, http.MethodDelete, "/_security/role/writer")
	if !tt.HasText("已删除角色 writer") {
		t.Errorf("missing success message; texts: %q", tt.Texts())
	}
}

func TestRolesOpenSearchNotSupported(t *testing.T) {
	_, fake, tt := newHarness(t, "2.14.0", "opensearch")

	if !tt.HasText("OpenSearch 不支持原生用户/角色管理") {
		t.Errorf("missing notice; texts: %q", tt.Texts())
	}
	if tt.HasText("reader") {
		t.Errorf("role list should not load on OpenSearch; texts: %q", tt.Texts())
	}
	if reqs := fake.all(); len(reqs) != 0 {
		t.Errorf("OpenSearch page made requests: %v", reqs)
	}
}
