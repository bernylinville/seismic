package nodes

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
)

// fakeHost runs Load synchronously so tests are deterministic.
type fakeHost struct {
	client *es.Client
	lang   string
}

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return "uuid-123" }
func (f *fakeHost) Version() string      { return "8.14.1" }
func (f *fakeHost) Distribution() string { return "elasticsearch" }
func (f *fakeHost) DB() *db.Store        { return nil }
func (f *fakeHost) BuildFlavor() string  { return "" }
func (f *fakeHost) Lang() string         { return f.lang }
func (f *fakeHost) Load(mark func(), fn func() func()) {
	mark()
	fn()()
}

const catNodesBody = `[
	{"name":"node-a","ip":"10.0.0.1","role":"dif","heap.percent":"41","ram.percent":"78","cpu":"3","load_1m":"0.42","disk.used_percent":"51","node.role":"data,ingest","version":"8.14.1"},
	{"name":"node-b","ip":"10.0.0.2","role":"mv","heap.percent":"96","ram.percent":"90","cpu":"87","load_1m":"4.10","disk.used_percent":"89","node.role":"voting_only","version":"8.14.1"}
]`

const nodesBody = `{"nodes":{"n1":{"name":"node-a","host":"10.0.0.1","attributes":{"rack":"r1","xpack.security.enabled":"true","ml.machine_memory":"16"}}}}`

func newHost(t *testing.T) *fakeHost {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/_cat/nodes":
			w.Write([]byte(catNodesBody))
		case "/_nodes":
			w.Write([]byte(nodesBody))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	client, err := es.New(srv.URL, es.Auth{Type: es.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	return &fakeHost{client: client}
}

func TestNodesRendersTable(t *testing.T) {
	host := newHost(t)
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1280, 800)

	for _, want := range []string{"node-a", "node-b", "10.0.0.1", "dif", "0.42", "data,ingest", "2 个节点"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q, texts %q", want, tt.Texts())
		}
	}
}

func TestNodesEnglishUI(t *testing.T) {
	host := newHost(t)
	host.lang = "en"
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1280, 800)
	for _, want := range []string{"Nodes", "2 nodes", "Hide system attributes"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q, texts %q", want, tt.Texts())
		}
	}
}

func TestNodesSortAndFilter(t *testing.T) {
	host := newHost(t)
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1280, 800)

	// 点击 CPU 表头排序：升序 node-a(3) 在前。
	if err := tt.Click("CPU"); err != nil {
		t.Fatalf("click CPU header: %v", err)
	}
	tt.Frame()
	if m.sort.Column != "CPU" || m.sort.Descending {
		t.Fatalf("sort = %+v, want CPU ascending", m.sort)
	}
	if rows := m.visibleRows(); rows[0].Name.String() != "node-a" || rows[1].Name.String() != "node-b" {
		t.Errorf("ascending CPU order = %s, %s", rows[0].Name, rows[1].Name)
	}

	// 再点一次反转为降序。
	if err := tt.Click("CPU"); err != nil {
		t.Fatalf("click CPU header again: %v", err)
	}
	tt.Frame()
	if !m.sort.Descending {
		t.Fatal("sort not descending after second click")
	}
	if rows := m.visibleRows(); rows[0].Name.String() != "node-b" {
		t.Errorf("descending CPU first row = %s, want node-b", rows[0].Name)
	}

	// 筛选：只匹配 node-b。
	m.filter = "node-b"
	tt.Frame()
	rows := m.visibleRows()
	if len(rows) != 1 || rows[0].Name.String() != "node-b" {
		t.Fatalf("filtered rows = %+v", rows)
	}
}

func TestNodesDetailHidesSystemAttributes(t *testing.T) {
	host := newHost(t)
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1280, 800)

	if err := tt.Click("node-a"); err != nil {
		t.Fatalf("click node-a: %v", err)
	}
	tt.Frame()

	if !tt.HasText("节点详情") {
		t.Fatalf("detail missing, texts %q", tt.Texts())
	}
	// 默认隐藏 ml./xpack./transform. 系统属性。
	if !strings.Contains(m.detailJSON, `"rack": "r1"`) {
		t.Errorf("rack attribute missing from detail: %s", m.detailJSON)
	}
	if strings.Contains(m.detailJSON, "xpack.security.enabled") || strings.Contains(m.detailJSON, "ml.machine_memory") {
		t.Errorf("system attributes visible while hidden: %s", m.detailJSON)
	}

	// 关闭开关后系统属性出现。
	if err := tt.Click("隐藏系统属性"); err != nil {
		t.Fatalf("toggle hide attrs: %v", err)
	}
	tt.Frame()
	if !strings.Contains(m.detailJSON, "xpack.security.enabled") {
		t.Errorf("system attributes still hidden: %s", m.detailJSON)
	}
}
