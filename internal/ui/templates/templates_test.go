package templates

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// fakeHost runs loads synchronously so tests stay deterministic.
type fakeHost struct {
	client *es.Client
	lang   string
}

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return "test" }
func (f *fakeHost) Version() string      { return "8.14.1" }
func (f *fakeHost) Distribution() string { return "elasticsearch" }
func (f *fakeHost) BuildFlavor() string  { return "default" }
func (f *fakeHost) DB() *db.Store        { return nil }
func (f *fakeHost) Lang() string         { return f.lang }
func (f *fakeHost) Load(key string, mark func(), fn func(context.Context) func()) {
	mark()
	fn(context.Background())()
}

const legacyJSON = `{
  "logs": {"order": 1, "index_patterns": ["logs-*"], "settings": {"number_of_shards": 1}},
  "metrics": {"order": 0, "index_patterns": ["metrics-*", "metricbeat-*"]}
}`

const composableJSON = `{
  "index_templates": [
    {"name": "logs-new", "index_patterns": ["logs-new-*"], "priority": 200},
    {"name": "metrics-new", "index_patterns": ["metrics-new-*"]}
  ]
}`

func newTemplateES(t *testing.T) (*httptest.Server, *map[string]string) {
	t.Helper()
	var mu sync.Mutex
	calls := map[string]string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		mu.Lock()
		calls[r.Method+" "+r.URL.Path] = "1"
		mu.Unlock()
		switch {
		case r.Method == "GET" && r.URL.Path == "/_template":
			w.Write([]byte(legacyJSON))
		case r.Method == "GET" && r.URL.Path == "/_index_template":
			w.Write([]byte(composableJSON))
		case r.Method == "GET" && r.URL.Path == "/_template/logs":
			w.Write([]byte(`{"logs": {"order": 1, "index_patterns": ["logs-*"]}}`))
		case r.Method == "DELETE" && r.URL.Path == "/_template/logs":
			w.Write([]byte(`{"acknowledged": true}`))
		case r.Method == "PUT" && r.URL.Path == "/_template/events":
			body := map[string]any{}
			json.NewDecoder(r.Body).Decode(&body)
			if pats, ok := body["index_patterns"].([]any); !ok || len(pats) != 1 || pats[0] != "events-*" {
				w.WriteHeader(400)
				w.Write([]byte(`{"error":"bad patterns"}`))
				return
			}
			w.Write([]byte(`{"acknowledged": true}`))
		default:
			w.Write([]byte(`{"acknowledged": true}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &calls
}

func newModule(t *testing.T) (*Module, *ui.Tester, *map[string]string) {
	t.Helper()
	srv, calls := newTemplateES(t)
	client, err := es.New(srv.URL, es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	m := New()
	host := &fakeHost{client: client}
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 640)
	return m, tt, calls
}

func called(calls *map[string]string, key string) bool {
	for k := range *calls {
		if k == key {
			return true
		}
	}
	return false
}

func TestTemplatesList(t *testing.T) {
	_, tt, _ := newModule(t)
	for _, want := range []string{"logs", "metrics", "logs-*", "metrics-*, metricbeat-*"} {
		if !tt.HasText(want) {
			t.Errorf("legacy list missing %q, texts %q", want, tt.Texts())
		}
	}
	if err := tt.Click("新版模板"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"logs-new", "metrics-new", "200"} {
		if !tt.HasText(want) {
			t.Errorf("composable list missing %q, texts %q", want, tt.Texts())
		}
	}
}

func TestTemplatesEnglishUI(t *testing.T) {
	srv, _ := newTemplateES(t)
	client, err := es.New(srv.URL, es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client, lang: "en"}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 640)
	for _, want := range []string{"Index Templates", "2 templates", "Index patterns", "Legacy templates"} {
		if !tt.HasText(want) {
			t.Errorf("english list missing %q, texts %q", want, tt.Texts())
		}
	}
}

func TestTemplatesDetailAndDelete(t *testing.T) {
	_, tt, calls := newModule(t)
	// 选中第一行（logs）后查看 JSON。
	if err := tt.Click("logs"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("查看 JSON"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText(`{"logs": {"order": 1, "index_patterns": ["logs-*"]}}`) {
		t.Errorf("detail JSON missing, texts %q", tt.Texts())
	}
	if err := tt.Click("关闭"); err != nil {
		t.Fatal(err)
	}

	if err := tt.Click("删除…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("确定删 logs？") {
		t.Errorf("confirm dialog missing, texts %q", tt.Texts())
	}
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	if !called(calls, "DELETE /_template/logs") {
		t.Errorf("DELETE not sent, calls %v", *calls)
	}
}

func TestTemplatesCreate(t *testing.T) {
	m, tt, calls := newModule(t)
	if err := tt.Click("新建…"); err != nil {
		t.Fatal(err)
	}
	tt.Type("events")
	m.createPats = "events-*"
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	if m.creating {
		t.Fatal("create dialog still open")
	}
	if !called(calls, "PUT /_template/events") {
		t.Errorf("PUT not sent, calls %v", *calls)
	}
}

var _ kit.ModuleHost = (*fakeHost)(nil)
