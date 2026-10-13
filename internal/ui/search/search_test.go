package search

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// fakeHost implements kit.ModuleHost with a synchronous Load.
type fakeHost struct {
	client *es.Client
	lang   string
}

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return "test" }
func (f *fakeHost) Version() string      { return "8.14.0" }
func (f *fakeHost) Distribution() string { return "elasticsearch" }
func (f *fakeHost) BuildFlavor() string  { return "" }
func (f *fakeHost) DB() *db.Store        { return nil }
func (f *fakeHost) Lang() string         { return f.lang }
func (f *fakeHost) Load(key string, mark func(), fn func(context.Context) func()) {
	mark()
	fn(context.Background())()
}

const testHits = `{"took":5,"hits":{"total":{"value":35},"hits":[
  {"_index":"logs-2024","_id":"1","_score":1.5,"_source":{"level":"info","host":{"name":"web-1"}}},
  {"_index":"logs-2024","_id":"2","_score":0.5,"_source":{"level":"error","host":{"name":"web-2"}}},
  {"_index":"metrics","_id":"3","_score":null,"_source":{"cpu":42}}
]}}`

// newFakeES starts an httptest ES recording search requests.
func newFakeES(t *testing.T) (*es.Client, *[]string) {
	t.Helper()
	var mu sync.Mutex
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "_cat/indices"):
			w.Write([]byte(`[{"index":"logs-2024"},{"index":"logs-2025"},{"index":"metrics"}]`))
		case strings.HasSuffix(r.URL.Path, "_search"):
			buf := make([]byte, r.ContentLength)
			r.Body.Read(buf)
			mu.Lock()
			bodies = append(bodies, r.URL.Path+" "+string(buf))
			mu.Unlock()
			w.Write([]byte(testHits))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	client, err := es.New(srv.URL, es.Auth{Type: es.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	return client, &bodies
}

func newModule(t *testing.T) (*Module, *ui.Tester, *[]string) {
	client, bodies := newFakeES(t)
	m := New()
	host := &fakeHost{client: client}
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)
	return m, tt, bodies
}

func TestSearchResultsTable(t *testing.T) {
	m, tt, _ := newModule(t)
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	// 表格列与行。
	for _, want := range []string{"_index", "_id", "_score", "level", "host.name", "cpu",
		"logs-2024", "metrics", "web-1", "42"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts: %q", want, tt.Texts())
		}
	}
	if m.total != 35 {
		t.Errorf("total = %d, want 35", m.total)
	}
	if len(m.rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(m.rows))
	}
	if m.rows[0].Score != "1.5" || m.rows[2].Score != "-" {
		t.Errorf("scores = %q, %q", m.rows[0].Score, m.rows[2].Score)
	}
	if !tt.HasText("共 35 条") {
		t.Errorf("missing total text; texts: %q", tt.Texts())
	}
}

func TestSearchEnglishUI(t *testing.T) {
	client, _ := newFakeES(t)
	host := &fakeHost{client: client, lang: "en"}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Document search", "35 hits", "Previous", "Next"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q; texts: %q", want, tt.Texts())
		}
	}
}

func TestSearchIndexPatternSent(t *testing.T) {
	m, tt, bodies := newModule(t)
	m.indexPattern = "log-*, metrics"
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	last := (*bodies)[len(*bodies)-1]
	if !strings.HasPrefix(last, "/log-*,metrics/_search") {
		t.Errorf("request path = %q", last)
	}
}

func TestSearchPagination(t *testing.T) {
	m, tt, bodies := newModule(t)
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	if got := bodyField(t, (*bodies)[0], "from"); got != 0 {
		t.Errorf("first page from = %d", got)
	}
	if err := tt.Click("下一页"); err != nil {
		t.Fatal(err)
	}
	if got := bodyField(t, (*bodies)[1], "from"); got != 10 {
		t.Errorf("second page from = %d", got)
	}
	if m.from != 10 {
		t.Errorf("m.from = %v", m.from)
	}
	if err := tt.Click("上一页"); err != nil {
		t.Fatal(err)
	}
	if got := bodyField(t, (*bodies)[2], "from"); got != 0 {
		t.Errorf("back to first page from = %d", got)
	}
}

func bodyField(t *testing.T, req string, field string) int {
	t.Helper()
	_, bodyText, _ := strings.Cut(req, " ")
	var body map[string]any
	if err := json.Unmarshal([]byte(bodyText), &body); err != nil {
		t.Fatalf("bad body %q: %v", req, err)
	}
	return int(body[field].(float64))
}

func TestFormatQuery(t *testing.T) {
	m, tt, _ := newModule(t)
	// 合法紧凑 JSON：格式化后带缩进换行。
	m.query = `{"query":{"match_all":{}},"size":5}`
	if err := tt.Click("格式化 JSON"); err != nil {
		t.Fatal(err)
	}
	if m.formatErr != "" || !strings.Contains(m.query, "\n  ") {
		t.Errorf("format err=%q query=%q", m.formatErr, m.query)
	}
	// 非法 JSON：保留原文并提示错误行。
	bad := "{\n  bad"
	m.query = bad
	if err := tt.Click("格式化 JSON"); err != nil {
		t.Fatal(err)
	}
	if m.query != bad || m.formatErr == "" || !strings.Contains(m.formatErr, "第 2 行") {
		t.Errorf("query=%q err=%q", m.query, m.formatErr)
	}
	// 执行非法查询也应提示而不是发请求。
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	if m.ranOnce {
		t.Errorf("search fired on invalid JSON")
	}
}

func TestReset(t *testing.T) {
	m, tt, _ := newModule(t)
	m.query = "{}"
	m.from, m.size = 40, 3
	if err := tt.Click("重置"); err != nil {
		t.Fatal(err)
	}
	if m.query != defaultQuery || m.from != 0 || m.size != 10 {
		t.Errorf("reset failed: %q %v %v", m.query, m.from, m.size)
	}
}

func TestRowDetail(t *testing.T) {
	m, tt, _ := newModule(t)
	if err := tt.Click("搜索"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("web-2"); err != nil {
		t.Fatal(err)
	}
	if m.selectedRow != 1 {
		t.Fatalf("selectedRow = %d, want 1", m.selectedRow)
	}
	if got := m.rows[1].prettySource(); !strings.Contains(got, "\"level\"") {
		t.Errorf("detail = %q", got)
	}
}

var _ kit.ModuleHost = (*fakeHost)(nil)
