package users

import (
	"context"
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
	lang         string
}

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return "test-cluster" }
func (f *fakeHost) Version() string      { return f.version }
func (f *fakeHost) Distribution() string { return f.distribution }
func (f *fakeHost) BuildFlavor() string  { return "" }
func (f *fakeHost) DB() *db.Store        { return nil }
func (f *fakeHost) Lang() string         { return f.lang }
func (f *fakeHost) Load(key string, mark func(), fn func(context.Context) func()) {
	mark()
	fn(context.Background())()
}

// fakeES records every request and answers the security endpoints.
type fakeES struct {
	mu       sync.Mutex
	requests []request
	users    string
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
	switch {
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/user"):
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, f.users)
	default:
		io.WriteString(w, `{}`)
	}
}

const testUsers = `{
  "elastic": {"roles":["superuser"],"full_name":"System","enabled":true,"metadata":{"_reserved":true}},
  "alice": {"roles":["kibana_admin","monitoring_user"],"full_name":"Alice","email":"alice@example.com","enabled":true},
  "bot": {"roles":["reader"],"enabled":false,"metadata":{"_deprecated":true,"_deprecated_reason":"use v2"}}
}`

// newHarness wires a module to a fake cluster and a headless view.
func newHarness(t *testing.T, version, distribution string) (*Module, *fakeES, *ui.Tester) {
	t.Helper()
	fake := &fakeES{users: testUsers}
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

func TestUsersListCurrentVersion(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	for _, want := range []string{"alice", "elastic", "bot", "kibana_admin, monitoring_user", "alice@example.com", "启用", "已弃用", "3 个用户"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts: %q", want, tt.Texts())
		}
	}
	fake.find(t, http.MethodGet, "/_security/user")
}

func TestUsersListOldVersion(t *testing.T) {
	_, fake, tt := newHarness(t, "6.5.0", "elasticsearch")

	if tt.HasText("已弃用") {
		t.Errorf("deprecated column should be hidden before 7.6; texts: %q", tt.Texts())
	}
	if !tt.HasText("启用") {
		t.Errorf("enabled column should show from 6 on; texts: %q", tt.Texts())
	}
	fake.find(t, http.MethodGet, "/_xpack/security/user")
}

func TestUsersOpenSearchNotSupported(t *testing.T) {
	_, fake, tt := newHarness(t, "2.14.0", "opensearch")

	if !tt.HasText("OpenSearch 不支持原生用户/角色管理") {
		t.Errorf("missing notice; texts: %q", tt.Texts())
	}
	if tt.HasText("alice") {
		t.Errorf("user list should not load on OpenSearch; texts: %q", tt.Texts())
	}
	if reqs := fake.all(); len(reqs) != 0 {
		t.Errorf("OpenSearch page made requests: %v", reqs)
	}
}

func TestUsersEnglishUI(t *testing.T) {
	fake := &fakeES{users: testUsers}
	srv := httptest.NewServer(http.HandlerFunc(fake.handler))
	t.Cleanup(srv.Close)
	client, err := es.New(srv.URL, es.Auth{Type: es.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	client.Version = "8.14.1"
	host := &fakeHost{client: client, version: "8.14.1", distribution: "elasticsearch", lang: "en"}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 960, 640)
	for _, want := range []string{"Users", "3 users", "Username", "New user", "Reserved"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts: %q", want, tt.Texts())
		}
	}
}

func TestCreateUser(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.Click("新建用户"); err != nil {
		t.Fatal(err)
	}
	tt.Type("carol")
	tt.Key(0, ui.KeyTab)
	tt.Type("secret123")
	tt.Key(0, ui.KeyTab)
	tt.Type("kibana_admin")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}

	req := fake.find(t, http.MethodPut, "/_security/user/carol")
	var body map[string]any
	if err := json.Unmarshal([]byte(req.body), &body); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	if body["password"] != "secret123" {
		t.Errorf("password = %v", body["password"])
	}
	if body["enabled"] != true {
		t.Errorf("enabled = %v, want true (version >= 6)", body["enabled"])
	}
	if roles, ok := body["roles"].([]any); !ok || len(roles) != 1 || roles[0] != "kibana_admin" {
		t.Errorf("roles = %v", body["roles"])
	}
	if !tt.HasText("已创建用户 carol") {
		t.Errorf("missing success message; texts: %q", tt.Texts())
	}
}

func TestCreateUserValidation(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.Click("新建用户"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("用户名不能为空") || !tt.HasText("密码不能为空") {
		t.Errorf("missing validation errors; texts: %q", tt.Texts())
	}
	for _, r := range fake.all() {
		if r.method == http.MethodPut {
			t.Errorf("invalid form must not be submitted: %v", r)
		}
	}
}

func TestEditUser(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.RightClick("alice"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("编辑…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("编辑用户") {
		t.Fatalf("edit dialog did not open; texts: %q", tt.Texts())
	}
	// Focus starts on the password field; one Tab reaches 角色.
	tt.Key(0, ui.KeyTab)
	tt.Type("kibana_admin")
	if err := tt.Click("保存"); err != nil {
		t.Fatal(err)
	}

	req := fake.find(t, http.MethodPut, "/_security/user/alice")
	var body map[string]any
	if err := json.Unmarshal([]byte(req.body), &body); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	if _, ok := body["password"]; ok {
		t.Errorf("empty password must not be sent: %v", body)
	}
	if roles, ok := body["roles"].([]any); !ok || len(roles) != 1 || roles[0] != "kibana_admin" {
		t.Errorf("roles = %v", body["roles"])
	}
	if !tt.HasText("已更新用户 alice") {
		t.Errorf("missing success message; texts: %q", tt.Texts())
	}
}

func TestReservedUserIsReadOnly(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.RightClick("elastic"); err != nil {
		t.Fatal(err)
	}
	menu := tt.Menu()
	for _, item := range menu {
		if item == "编辑…" || item == "删除…" {
			t.Errorf("reserved user offers %q; menu: %q", item, menu)
		}
	}
	if err := tt.ChooseMenuItem("修改密码…"); err != nil {
		t.Fatal(err)
	}
	tt.Type("new-secret")
	if err := tt.Click("修改"); err != nil {
		t.Fatal(err)
	}

	req := fake.find(t, http.MethodPut, "/_security/user/elastic/_password")
	var body map[string]any
	if err := json.Unmarshal([]byte(req.body), &body); err != nil {
		t.Fatalf("PUT body: %v", err)
	}
	if body["password"] != "new-secret" {
		t.Errorf("password = %v", body["password"])
	}
}

func TestDeleteUser(t *testing.T) {
	_, fake, tt := newHarness(t, "8.14.1", "elasticsearch")

	if err := tt.RightClick("alice"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("删除…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("删了就恢复不了，确定删 alice？") {
		t.Fatalf("confirm dialog did not open; texts: %q", tt.Texts())
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}

	fake.find(t, http.MethodDelete, "/_security/user/alice")
	if !tt.HasText("已删除用户 alice") {
		t.Errorf("missing success message; texts: %q", tt.Texts())
	}
}
