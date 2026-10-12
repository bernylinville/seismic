package repositories

import (
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
)

// fakeHost runs Load synchronously, for deterministic view tests.
type fakeHost struct {
	client  *es.Client
	version string
}

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return "test-cluster" }
func (f *fakeHost) Version() string      { return f.version }
func (f *fakeHost) Distribution() string { return "elasticsearch" }
func (f *fakeHost) BuildFlavor() string  { return "" }
func (f *fakeHost) DB() *db.Store        { return nil }
func (f *fakeHost) Load(mark func(), fn func() func()) {
	mark()
	fn()()
}

// fakeES records every request and answers fixed JSON by path.
type fakeES struct {
	mu     sync.Mutex
	calls  []string
	bodies []string
}

func (f *fakeES) record(method, path, body string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, method+" "+path)
	f.bodies = append(f.bodies, body)
}

func (f *fakeES) has(call string) bool {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, c := range f.calls {
		if c == call {
			return true
		}
	}
	return false
}

func (f *fakeES) lastBody() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.bodies) == 0 {
		return ""
	}
	return f.bodies[len(f.bodies)-1]
}

func newFakeES(t *testing.T) (*fakeES, *fakeHost) {
	t.Helper()
	f := &fakeES{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.record(r.Method, r.URL.Path, string(body))
		switch {
		case r.URL.Path == "/_snapshot":
			io.WriteString(w, `{"repo1":{"type":"fs","settings":{"location":"/mnt/backup"}},"repo2":{"type":"s3","settings":{"bucket":"b"}}}`)
		default:
			io.WriteString(w, `{"acknowledged":true}`)
		}
	}))
	t.Cleanup(server.Close)
	client, err := es.New(server.URL, es.Auth{Type: es.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	return f, &fakeHost{client: client, version: "8.14.1"}
}

func TestRepositoriesListAndVerify(t *testing.T) {
	_, host := newFakeES(t)
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	for _, text := range []string{"repo1", "repo2", "fs", "s3", "/mnt/backup"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %q", text, tt.Texts())
		}
	}

	if err := tt.Click("验证"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("repo1 验证通过") {
		t.Fatalf("verify message missing: %q", tt.Texts())
	}
}

func TestRepositoriesCreate(t *testing.T) {
	f, host := newFakeES(t)
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	if err := tt.Click("新建仓库…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("新建快照仓库") {
		t.Fatalf("create dialog missing: %q", tt.Texts())
	}
	if err := tt.Click("仓库名称"); err != nil {
		t.Fatal(err)
	}
	tt.Type("myrepo")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	if !f.has("PUT /_snapshot/myrepo") {
		t.Fatalf("PUT not sent: %q", f.calls)
	}
	if m.showNew {
		t.Fatal("dialog still open after create")
	}
}

func TestRepositoriesDelete(t *testing.T) {
	f, host := newFakeES(t)
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	if err := tt.Click("删除…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("删除仓库") {
		t.Fatalf("confirm dialog missing: %q", tt.Texts())
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	if !f.has("DELETE /_snapshot/repo1") {
		t.Fatalf("DELETE not sent: %q", f.calls)
	}
}
