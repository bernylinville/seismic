package settings

import (
	"context"
	"encoding/json"
	"io"
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

const settingsJSON = `{
  "persistent": {
    "cluster": {"max_shards_per_node": 1000},
    "action": {"auto_create_index": "true"}
  },
  "transient": {"cluster": {"routing": {"allocation": {"enable": "all"}}}}
}`

func newSettingsES(t *testing.T) (*httptest.Server, *map[string][]byte) {
	t.Helper()
	var mu sync.Mutex
	puts := map[string][]byte{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == "GET" && r.URL.Path == "/_cluster/settings":
			w.Write([]byte(settingsJSON))
		case r.Method == "PUT" && r.URL.Path == "/_cluster/settings":
			body, _ := io.ReadAll(r.Body)
			mu.Lock()
			puts["last"] = body
			mu.Unlock()
			w.Write([]byte(`{"acknowledged": true}`))
		default:
			w.Write([]byte(`{}`))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &puts
}

func newModule(t *testing.T) (*Module, *ui.Tester, *map[string][]byte) {
	t.Helper()
	srv, puts := newSettingsES(t)
	client, err := es.New(srv.URL, es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	m := New()
	host := &fakeHost{client: client, lang: ""}
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 640)
	return m, tt, puts
}

func TestSettingsEnglishUI(t *testing.T) {
	srv, _ := newSettingsES(t)
	client, err := es.New(srv.URL, es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client, lang: "en"}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 640)
	for _, want := range []string{"Cluster settings", "3 settings", "Group", "Key", "Value"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q, texts %q", want, tt.Texts())
		}
	}
}

func TestSettingsList(t *testing.T) {
	_, tt, _ := newModule(t)
	for _, want := range []string{
		"永久", "临时",
		"cluster.max_shards_per_node", "1000",
		"action.auto_create_index", "true",
		"cluster.routing.allocation.enable", "all",
	} {
		if !tt.HasText(want) {
			t.Errorf("settings list missing %q, texts %q", want, tt.Texts())
		}
	}
}

func TestSettingsEditRequiresConfirm(t *testing.T) {
	m, tt, puts := newModule(t)
	if err := tt.Click("cluster.max_shards_per_node"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("编辑…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("当前值：1000") {
		t.Errorf("edit dialog missing current value, texts %q", tt.Texts())
	}
	if err := tt.Click("应用…"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("确定改吗") {
		t.Errorf("confirm step missing, texts %q", tt.Texts())
	}
	if len((*puts)["last"]) != 0 {
		t.Fatalf("PUT sent before confirmation: %s", (*puts)["last"])
	}
	if err := tt.Click("确认修改"); err != nil {
		t.Fatal(err)
	}
	if m.editing {
		t.Fatal("edit dialog still open")
	}
	body := (*puts)["last"]
	if len(body) == 0 {
		t.Fatal("no PUT recorded")
	}
	var doc map[string]any
	if err := json.Unmarshal(body, &doc); err != nil {
		t.Fatalf("PUT body invalid: %s", body)
	}
	inner := doc["persistent"].(map[string]any)
	if inner["cluster.max_shards_per_node"] != float64(1000) {
		t.Errorf("PUT body = %s, want persistent cluster.max_shards_per_node 1000", body)
	}
}

func TestSettingsEditTransient(t *testing.T) {
	m, tt, puts := newModule(t)
	// transient 行的默认开关是 transient。
	if err := tt.Click("cluster.routing.allocation.enable"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("编辑…"); err != nil {
		t.Fatal(err)
	}
	if !m.transient {
		t.Fatal("transient checkbox should default to the row's section")
	}
	m.editValue = ""
	tt.Command("selectAll")
	tt.Type(`"none"`)
	if err := tt.Click("应用…"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("确认修改"); err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal((*puts)["last"], &doc); err != nil {
		t.Fatal(err)
	}
	inner, ok := doc["transient"].(map[string]any)
	if !ok || inner["cluster.routing.allocation.enable"] != "none" {
		t.Errorf("PUT body = %s, want transient cluster.routing.allocation.enable \"none\"", (*puts)["last"])
	}
}

var _ kit.ModuleHost = (*fakeHost)(nil)
