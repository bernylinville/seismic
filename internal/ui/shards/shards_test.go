package shards

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

const shardsJSON = `[
 {"index":"logs-2024","shard":0,"prirep":"p","state":"STARTED","docs":120,"store":"5kb","node":"node-a","unassigned.reason":""},
 {"index":"logs-old","shard":1,"prirep":"r","state":"UNASSIGNED","docs":0,"store":"","node":"","unassigned.reason":"INDEX_CREATED"},
 {"index":"metrics","shard":0,"prirep":"p","state":"INITIALIZING","docs":3,"store":"1kb","node":"node-b","unassigned.reason":""}
]`

func newShardsServer(t *testing.T, rerouteBody *string, rerouteMu *sync.Mutex) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/_cluster/reroute":
			b, _ := io.ReadAll(r.Body)
			rerouteMu.Lock()
			*rerouteBody = string(b)
			rerouteMu.Unlock()
			w.Write([]byte(`{"acknowledged":true}`))
		case strings.HasPrefix(r.URL.Path, "/_cat/shards"):
			if got := r.URL.Query().Get("h"); got != "index,shard,prirep,state,docs,store,node,unassigned.reason" {
				t.Errorf("h param = %q", got)
			}
			if r.URL.Query().Get("format") != "json" {
				t.Errorf("format param missing")
			}
			w.Write([]byte(shardsJSON))
		default:
			t.Errorf("unexpected path %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
}

func TestShardsView(t *testing.T) {
	var mu sync.Mutex
	body := ""
	srv := newShardsServer(t, &body, &mu)
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	for _, want := range []string{"logs-2024", "metrics", "STARTED", "UNASSIGNED", "INITIALIZING", "INDEX_CREATED", "node-a", "3 个分片"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
}

func TestShardsEnglishUI(t *testing.T) {
	var mu sync.Mutex
	body := ""
	srv := newShardsServer(t, &body, &mu)
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client, lang: "en"}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)
	for _, want := range []string{"Shards", "3 shards", "Reason"} {
		if !tt.HasText(want) {
			t.Errorf("missing text %q; texts %q", want, tt.Texts())
		}
	}
}

func TestShardsRerouteUnassigned(t *testing.T) {
	var mu sync.Mutex
	body := ""
	srv := newShardsServer(t, &body, &mu)
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	if err := tt.RightClick("logs-old"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("移到节点…"); err != nil {
		t.Fatal(err)
	}
	// 未分配副本走 allocate_replica 命令。
	if err := tt.Click("node-a"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(body, "allocate_replica") || !strings.Contains(body, `"index":"logs-old"`) || !strings.Contains(body, `"node":"node-a"`) {
		t.Errorf("reroute body = %s", body)
	}
}

func TestShardsRerouteMove(t *testing.T) {
	var mu sync.Mutex
	body := ""
	srv := newShardsServer(t, &body, &mu)
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

	if err := tt.RightClick("metrics"); err != nil {
		t.Fatal(err)
	}
	if err := tt.ChooseMenuItem("重新分配…"); err != nil {
		t.Fatal(err)
	}
	if err := tt.Click("node-a"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if !strings.Contains(body, "move") || !strings.Contains(body, `"from_node":"node-b"`) || !strings.Contains(body, `"to_node":"node-a"`) {
		t.Errorf("reroute body = %s", body)
	}
}

func TestShardsFilter(t *testing.T) {
	var mu sync.Mutex
	body := ""
	srv := newShardsServer(t, &body, &mu)
	defer srv.Close()

	client, err := es.New(srv.URL+"/", es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	host := &fakeHost{client: client}
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 900, 600)

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

// TestVisibleRowsCached checks the visible-rows cache: unchanged inputs reuse
// the computed slice, input changes recompute.
func TestVisibleRowsCached(t *testing.T) {
	m := New()
	m.rows = []Row{
		{Index: "logs-2024", Shard: "0", Prirep: "p", State: "STARTED"},
		{Index: "metrics", Shard: "0", Prirep: "p", State: "STARTED"},
	}

	r1 := m.visibleRows()
	r2 := m.visibleRows()
	if r1[0].Index != "logs-2024" || len(r1) != 2 {
		t.Fatalf("visibleRows = %+v", r1)
	}
	if &r1[0] != &r2[0] {
		t.Error("unchanged inputs should reuse the cached slice")
	}

	m.filter = "MET" // filter lowercases both sides
	if got := m.visibleRows(); len(got) != 1 || got[0].Index != "metrics" {
		t.Errorf("filter change not applied: %+v", got)
	}

	m.filter = ""
	m.sort = ui.SortOrder{Descending: true}
	if got := m.visibleRows(); got[0].Index != "metrics" {
		t.Errorf("sort change not applied: %+v", got)
	}

	m.sort = ui.SortOrder{}
	m.rows = []Row{{Index: "b"}, {Index: "a"}}
	if got := m.visibleRows(); got[0].Index != "a" {
		t.Errorf("rows swap left a stale cache: %+v", got)
	}
}
