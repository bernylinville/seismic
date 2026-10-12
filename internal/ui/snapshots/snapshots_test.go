package snapshots

import (
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

func (f *fakeES) bodyOf(call string) string {
	f.mu.Lock()
	defer f.mu.Unlock()
	for i, c := range f.calls {
		if c == call {
			return f.bodies[i]
		}
	}
	return ""
}

const snapJSON = `{"snapshots":[{"snapshot":"snap1","state":"SUCCESS","start_time_in_millis":1700000000000,"end_time_in_millis":1700000060000,"indices":["index_b","index_a"],"shards":{"total":4,"failed":0,"successful":4}}]}`

func newFakeES(t *testing.T, version string) (*fakeES, *fakeHost) {
	t.Helper()
	f := &fakeES{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		f.record(r.Method, r.URL.Path, string(body))
		switch {
		case r.URL.Path == "/_snapshot":
			io.WriteString(w, `{"repo1":{"type":"fs","settings":{"location":"/mnt/backup"}}}`)
		case r.URL.Path == "/_snapshot/repo1/_all", r.URL.Path == "/_snapshot/repo1/snap1":
			io.WriteString(w, snapJSON)
		case r.URL.Path == "/_slm/policy":
			io.WriteString(w, `{"daily-snapshots":{"policy":{"name":"daily","schedule":"0 30 1 * * ?","repository":"repo1"},"next_execution_millis":1700000000000}}`)
		case r.URL.Path == "/_slm/status":
			io.WriteString(w, `{"operation_mode":"RUNNING"}`)
		default:
			io.WriteString(w, `{"acknowledged":true}`)
		}
	}))
	t.Cleanup(server.Close)
	client, err := es.New(server.URL, es.Auth{Type: es.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	return f, &fakeHost{client: client, version: version}
}

func TestSnapshotsListAndDetail(t *testing.T) {
	_, host := newFakeES(t, "8.14.1")
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	for _, text := range []string{"snap1", "SUCCESS"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %q", text, tt.Texts())
		}
	}

	if err := tt.Click("详情…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("index_a") || !tt.HasText("index_b") {
		t.Fatalf("detail indices missing: %q", tt.Texts())
	}
	if !tt.HasText("索引 2 个") {
		t.Fatalf("detail counts missing: %q", tt.Texts())
	}
	if !tt.HasText("分片 成功 4 / 失败 0 / 总数 4") {
		t.Fatalf("detail shards missing: %q", tt.Texts())
	}
}

func TestSnapshotsRestore(t *testing.T) {
	f, host := newFakeES(t, "8.14.1")
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	if err := tt.Click("恢复…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("恢复快照 snap1") {
		t.Fatalf("restore dialog missing: %q", tt.Texts())
	}
	if err := tt.Click("重命名模式"); err != nil {
		t.Fatal(err)
	}
	tt.Type("index_(.+)")
	if err := tt.Click("替换为"); err != nil {
		t.Fatal(err)
	}
	tt.Type("restored_$1")
	if err := tt.Click("恢复"); err != nil {
		t.Fatal(err)
	}
	body := f.bodyOf("POST /_snapshot/repo1/snap1/_restore")
	if body == "" {
		t.Fatalf("restore not sent: %q", f.calls)
	}
	for _, want := range []string{`"rename_pattern":"index_(.+)"`, `"rename_replacement":"restored_$1"`} {
		if !contains(body, want) {
			t.Fatalf("restore body %q missing %q", body, want)
		}
	}
	if m.restoreOpen {
		t.Fatal("restore dialog still open")
	}
}

func TestSnapshotsCreateAndDelete(t *testing.T) {
	f, host := newFakeES(t, "8.14.1")
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	if err := tt.Click("新建快照…"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("快照名称"); err != nil {
		t.Fatal(err)
	}
	tt.Type("snap2")
	if err := tt.Click("索引"); err != nil {
		t.Fatal(err)
	}
	tt.Type("index_a")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	body := f.bodyOf("PUT /_snapshot/repo1/snap2")
	if body == "" {
		t.Fatalf("create not sent: %q", f.calls)
	}
	if !contains(body, `"indices":["index_a"]`) {
		t.Fatalf("create body %q missing indices", body)
	}

	if err := tt.Click("删除快照…"); err != nil {
		t.Fatal(err)
	}
	if !f.has("DELETE /_snapshot/repo1/snap1") {
		t.Fatalf("delete not sent: %q", f.calls)
	}
}

func TestSlmSectionShownOnSupportedVersion(t *testing.T) {
	f, host := newFakeES(t, "7.17.0")
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	for _, text := range []string{"SLM 策略", "daily-snapshots", "RUNNING", "0 30 1 * * ?"} {
		if !tt.HasText(text) {
			t.Fatalf("missing %q in %q", text, tt.Texts())
		}
	}

	if err := tt.Click("执行"); err != nil {
		t.Fatal(err)
	}
	if !f.has("POST /_slm/policy/daily-snapshots/_execute") {
		t.Fatalf("execute not sent: %q", f.calls)
	}

	if err := tt.Click("新建策略…"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("策略名称"); err != nil {
		t.Fatal(err)
	}
	tt.Type("weekly")
	if err := tt.Click("调度 schedule"); err != nil {
		t.Fatal(err)
	}
	tt.Type("0 0 2 * * ?")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	body := f.bodyOf("PUT /_slm/policy/weekly")
	if body == "" {
		t.Fatalf("policy create not sent: %q", f.calls)
	}
	if !contains(body, `"repository":"repo1"`) {
		t.Fatalf("policy body %q missing repository", body)
	}

	if err := tt.Click("删除策略…"); err != nil {
		t.Fatal(err)
	}
	if !f.has("DELETE /_slm/policy/daily-snapshots") {
		t.Fatalf("policy delete not sent: %q", f.calls)
	}
}

func TestSlmSectionHiddenBelow74(t *testing.T) {
	f, host := newFakeES(t, "6.8.0")
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	if tt.HasText("SLM 策略") || tt.HasText("daily-snapshots") {
		t.Fatalf("SLM section visible on 6.8.0: %q", tt.Texts())
	}
	for _, call := range f.calls {
		if strings.Contains(call, "_slm") {
			t.Fatalf("SLM request sent on 6.8.0: %q", f.calls)
		}
	}
	if !tt.HasText("snap1") {
		t.Fatalf("snapshots missing: %q", tt.Texts())
	}
}

func contains(s, sub string) bool { return strings.Contains(s, sub) }
