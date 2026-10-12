package recovery

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
)

// fakeHost is a synchronous ModuleHost over an httptest ES.
type fakeHost struct {
	client *es.Client
	lang   string
}

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return "test" }
func (f *fakeHost) Version() string      { return "8.14.1" }
func (f *fakeHost) Distribution() string { return "elasticsearch" }
func (f *fakeHost) BuildFlavor() string  { return "" }
func (f *fakeHost) DB() *db.Store        { return nil }
func (f *fakeHost) Lang() string         { return f.lang }
func (f *fakeHost) Load(mark func(), fn func() func()) {
	mark()
	apply := fn()
	apply()
}

const recoveryJSON = `[
 {"index":"logs-2024","shard":"0","type":"store","stage":"DONE","source":"node-a","node":"node-b","start_time":"1700000000000"},
 {"index":"metrics","shard":"1","type":"snapshot","stage":"TRANSLOG","source":"repo/snap-1","node":"node-a","start_time":"1700000001000"}
]`

func newRecoveryServer(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/_cat/recovery" {
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
			return
		}
		q := r.URL.Query()
		if q.Get("h") != "index,shard,type,stage,source,node,start_time" {
			t.Errorf("h param = %q", q.Get("h"))
		}
		if q.Get("s") != "start_time_millis:desc" {
			t.Errorf("s param = %q", q.Get("s"))
		}
		if q.Get("format") != "json" {
			t.Errorf("format param missing")
		}
		w.Write([]byte(recoveryJSON))
	}))
}

func TestRecoveryView(t *testing.T) {
	srv := newRecoveryServer(t)
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	for _, want := range []string{"logs-2024", "metrics", "DONE", "TRANSLOG", "snapshot", "repo/snap-1", "node-a", "2 条恢复记录"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
}

func TestRecoveryEnglishUI(t *testing.T) {
	srv := newRecoveryServer(t)
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client, lang: "en"}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)
	for _, want := range []string{"Shard Recovery", "2 recoveries", "Stage", "Source", "Started"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
}

func TestRecoveryRefreshAndFilter(t *testing.T) {
	srv := newRecoveryServer(t)
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	if err := tt.Click("刷新"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("logs-2024") {
		t.Errorf("refresh lost rows; texts %q", tt.Texts())
	}

	if err := tt.Click("筛选索引…"); err != nil {
		t.Fatal(err)
	}
	tt.Type("metrics")
	if tt.HasText("logs-2024") {
		t.Errorf("logs-2024 should be filtered out; texts %q", tt.Texts())
	}
	if !tt.HasText("metrics") {
		t.Errorf("metrics missing; texts %q", tt.Texts())
	}
}

func TestRecoveryEmpty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`[]`))
	}))
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	if !tt.HasText("没有恢复记录") {
		t.Errorf("empty state missing; texts %q", tt.Texts())
	}
	if strings.Contains(strings.Join(tt.Texts(), " "), "undefined") {
		t.Errorf("rendered undefined")
	}
}
