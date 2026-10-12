package overview

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
)

// fakeHost runs Load synchronously so tests are deterministic.
type fakeHost struct {
	client  *es.Client
	cluster string
}

func (f *fakeHost) ES() *es.Client       { return f.client }
func (f *fakeHost) ClusterKey() string   { return f.cluster }
func (f *fakeHost) Version() string      { return "8.14.1" }
func (f *fakeHost) Distribution() string { return "elasticsearch" }
func (f *fakeHost) BuildFlavor() string  { return "" }
func (f *fakeHost) DB() *db.Store        { return nil }
func (f *fakeHost) Load(mark func(), fn func() func()) {
	mark()
	fn()()
}

func newHost(t *testing.T, statsCalls *atomic.Int32) *fakeHost {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/":
			w.Write([]byte(`{"cluster_name":"测试集群","cluster_uuid":"uuid-123","version":{"number":"8.14.1","distribution":"elasticsearch"}}`))
		case "/_cluster/health":
			w.Write([]byte(`{"status":"yellow","active_shards":12,"unassigned_shards":3,"number_of_nodes":2}`))
		case "/_cluster/stats":
			statsCalls.Add(1)
			w.Write([]byte(`{"status":"yellow","nodes":{"count":{"total":2,"master":1,"data":2}},
				"indices":{"count":7,"shards":{"total":15,"primaries":12},
				"docs":{"count":1234567},"store":{"size_in_bytes":5368709120}}}`))
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	client, err := es.New(srv.URL, es.Auth{Type: es.AuthNone})
	if err != nil {
		t.Fatal(err)
	}
	return &fakeHost{client: client, cluster: "uuid-123"}
}

func TestOverviewRendersCluster(t *testing.T) {
	var statsCalls atomic.Int32
	host := newHost(t, &statsCalls)
	m := New()
	tt := ui.NewTester(func(c *ui.Context) { m.View(host, c) }, 1280, 800)

	for _, want := range []string{
		"测试集群", "uuid-123", "8.14.1", "elasticsearch",
		"yellow", "活动分片 12", "未分配分片 3", "1 master · 2 data",
		"1234567", "5.0 GB 磁盘占用", "7 个索引",
	} {
		if !tt.HasText(want) {
			t.Errorf("missing %q, texts %q", want, tt.Texts())
		}
	}
	if n := statsCalls.Load(); n != 1 {
		t.Errorf("stats calls = %d, want 1", n)
	}

	// 手动刷新再次拉取。
	if err := tt.Click("刷新"); err != nil {
		t.Fatalf("click refresh: %v", err)
	}
	tt.Frame()
	if n := statsCalls.Load(); n != 2 {
		t.Errorf("stats calls after refresh = %d, want 2", n)
	}
}

func TestHumanBytes(t *testing.T) {
	for b, want := range map[float64]string{
		512:           "512 B",
		2048:          "2.0 KB",
		5368709120:    "5.0 GB",
		5 * 1024:      "5.0 KB",
		1073741824:    "1.0 GB",
		1099511627776: "1.0 TB",
	} {
		if got := humanBytes(b); got != want {
			t.Errorf("humanBytes(%v) = %q, want %q", b, got, want)
		}
	}
}
