package rest

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/egoist/mygo/ui"
	"seismic/internal/db"
	"seismic/internal/es"
	"seismic/internal/ui/kit"
	"strings"
)

// fakeHost is a kit.ModuleHost over an httptest ES and a real SQLite store.
type fakeHost struct {
	client *es.Client
	key    string
	store  *db.Store
	lang   string
}

func (f *fakeHost) Lang() string { return f.lang }

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return f.key }
func (f *fakeHost) Version() string      { return "8.14.1" }
func (f *fakeHost) Distribution() string { return "elasticsearch" }
func (f *fakeHost) BuildFlavor() string  { return "" }
func (f *fakeHost) DB() *db.Store        { return f.store }
func (f *fakeHost) Load(mark func(), fn func() func()) {
	mark()
	if apply := fn(); apply != nil {
		apply()
	}
}

func newFakeHost(t *testing.T, key string, store *db.Store, handler http.HandlerFunc) *fakeHost {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	client, err := es.New(srv.URL, es.Auth{})
	if err != nil {
		t.Fatal(err)
	}
	return &fakeHost{client: client, key: key, store: store}
}

func newStore(t *testing.T) *db.Store {
	t.Helper()
	store, err := db.Open(context.Background(), t.TempDir()+"/test.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestSendShowsResponseAndHistory(t *testing.T) {
	store := newStore(t)
	host := newFakeHost(t, "cluster-a", store, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"acknowledged":true}`))
	})
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("200") {
		t.Errorf("missing status 200 in %q", tt.Texts())
	}
	if !tt.HasText("耗时") {
		t.Errorf("missing duration in %q", tt.Texts())
	}
	if !strings.Contains(m.tabs[0].resp, "acknowledged") {
		t.Errorf("response body not stored: %q", m.tabs[0].resp)
	}
	// 历史面板应出现这条请求。
	if !tt.HasText("_cat/indices") {
		t.Errorf("missing history path in %q", tt.Texts())
	}
	entries, err := store.ListHistory(context.Background(), "cluster-a", 10, 0)
	if err != nil || len(entries) != 1 {
		t.Fatalf("history in db: %d err %v", len(entries), err)
	}
	if entries[0].Status != 200 || entries[0].Path != "_cat/indices" {
		t.Errorf("bad entry: %+v", entries[0])
	}
}

func TestHistoryClickRefillsTab(t *testing.T) {
	store := newStore(t)
	host := newFakeHost(t, "cluster-a", store, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}

	// 把路径改成别的，再点历史条目回填。
	if err := tt.Click("路径"); err != nil {
		t.Fatal(err)
	}
	tt.Command("selectAll")
	tt.Type("/_cluster/health")
	if m.tabs[0].path != "/_cluster/health" {
		t.Fatalf("typing path failed: %q", m.tabs[0].path)
	}

	if err := tt.Click("_cat/indices"); err != nil {
		t.Fatal(err)
	}
	if m.tabs[0].path != "_cat/indices" || m.tabs[0].method != "GET" {
		t.Errorf("history click did not refill: %+v", m.tabs[0])
	}
}

func TestHistoryIsolatedPerCluster(t *testing.T) {
	store := newStore(t)
	host := newFakeHost(t, "cluster-a", store, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	other := &fakeHost{client: host.client, key: "cluster-b", store: store}

	m := New()
	hostPtr := &host
	tt := ui.NewTester(func(c *ui.Context) { m.View(*hostPtr, c) }, 1000, 700)

	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	if len(m.history) != 1 {
		t.Fatalf("cluster-a history: %d", len(m.history))
	}

	*hostPtr = other
	tt.Frame()
	if len(m.history) != 0 {
		t.Errorf("cluster-b should see no history, got %d", len(m.history))
	}
	if m.tabs[0].sent {
		t.Errorf("response must not leak across clusters")
	}
}

func TestTabsAndSavedQueries(t *testing.T) {
	store := newStore(t)
	host := newFakeHost(t, "cluster-a", store, func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{}`))
	})
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	if len(m.tabs) != 1 {
		t.Fatalf("tabs: %d", len(m.tabs))
	}
	if err := tt.Click("新建标签"); err != nil {
		t.Fatal(err)
	}
	if len(m.tabs) != 2 || m.active != 1 {
		t.Fatalf("after add: tabs %d active %d", len(m.tabs), m.active)
	}
	if err := tt.Click("关闭标签"); err != nil {
		t.Fatal(err)
	}
	if len(m.tabs) != 1 {
		t.Fatalf("after close: %d", len(m.tabs))
	}

	// 保存当前查询，然后改路径，再点保存的查询回填。
	if err := tt.Click("查询名称"); err != nil {
		t.Fatal(err)
	}
	tt.Type("我的查询")
	if err := tt.Click("保存查询"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("我的查询") {
		t.Errorf("saved query not listed: %q", tt.Texts())
	}
	m.tabs[0].path = "/_other"
	if err := tt.Click("我的查询"); err != nil {
		t.Fatal(err)
	}
	if m.tabs[0].path != "_cat/indices" {
		t.Errorf("saved query did not refill: %q", m.tabs[0].path)
	}

	// 删除保存的查询。
	if err := tt.Click("删除"); err != nil {
		t.Fatal(err)
	}
	if len(m.saved) != 0 {
		t.Errorf("saved query not deleted: %d", len(m.saved))
	}
}

func TestErrorResponse(t *testing.T) {
	store := newStore(t)
	host := newFakeHost(t, "cluster-a", store, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"boom"}`))
	})
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)

	if err := tt.Click("发送"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("500") {
		t.Errorf("missing status 500 in %q", tt.Texts())
	}
	if !tt.HasText("boom") {
		t.Errorf("missing error body in %q", tt.Texts())
	}
}

func TestRestEnglishUI(t *testing.T) {
	store := newStore(t)
	host := newFakeHost(t, "cluster-a", store, func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`{"ok":true}`))
	})
	host.lang = "en"
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1000, 700)
	if err := tt.Click("Send"); err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"Console", "History", "Saved queries", "New tab", "ms"} {
		if !tt.HasText(want) {
			t.Errorf("missing %q in %q", want, tt.Texts())
		}
	}
}

var _ kit.ModuleHost = (*fakeHost)(nil)
