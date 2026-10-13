package main

import (
	"context"
	"fmt"
	"image/png"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"

	"seismic/internal/conn"
	"seismic/internal/db"
	appui "seismic/internal/ui"
)

// TestRenderSmokeScreens renders the real UI headless and writes PNGs for
// visual review into SHOTS_DIR (default ../es-native-design/shots/ui/).
func TestRenderSmokeScreens(t *testing.T) {
	outDir := os.Getenv("SHOTS_DIR")
	if outDir == "" {
		outDir = filepath.Join("..", "es-native-design", "shots", "ui")
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/":
			w.Write([]byte(`{"cluster_name":"prod-es-8","cluster_uuid":"abc123","version":{"number":"8.14.1","distribution":"elasticsearch","build_flavor":"default"}}`))
		case r.URL.Path == "/_cluster/health":
			w.Write([]byte(`{"status":"green"}`))
		case r.URL.Path == "/_cluster/stats":
			w.Write([]byte(`{"_nodes":{"total":3,"successful":3},"nodes":{"count":{"total":3,"data":2,"master":1}},"indices":{"count":6,"docs":{"count":178199002},"shards":{"total":34,"primaries":19},"store":{"size_in_bytes":201005878272}}}`))
		case r.URL.Path == "/_cat/nodes" || r.URL.Path == "/_cat/nodes/":
			w.Write([]byte(`[{"name":"es-01","ip":"10.0.0.1","heap.percent":"41","ram.percent":"62","cpu":"7","load_1m":"0.42","disk.used_percent":"58","node.role":"cdfhilmrstw","version":"8.14.1"},{"name":"es-02","ip":"10.0.0.2","heap.percent":"38","ram.percent":"55","cpu":"3","load_1m":"0.21","disk.used_percent":"51","node.role":"dfhilmrstw","version":"8.14.1"},{"name":"es-03","ip":"10.0.0.3","heap.percent":"44","ram.percent":"67","cpu":"9","load_1m":"0.55","disk.used_percent":"61","node.role":"dfhilmrstw","version":"8.14.1"}]`))
		case len(r.URL.Path) >= len("/_cat/indices") && r.URL.Path[:len("/_cat/indices")] == "/_cat/indices":
			w.Write([]byte(`[
{"index":"logs-2026.10.12","health":"green","status":"open","docs.count":"12482913","store.size":"8.4gb","pri":"3","rep":"2","creation.date.string":"2026-10-12"},
{"index":"logs-2026.10.11","health":"green","status":"open","docs.count":"11930004","store.size":"8.1gb","pri":"3","rep":"2","creation.date.string":"2026-10-11"},
{"index":"metrics-2026.10","health":"green","status":"open","docs.count":"148220431","store.size":"96.2gb","pri":"5","rep":"1","creation.date.string":"2026-10-01"},
{"index":"orders-2026","health":"yellow","status":"open","docs.count":"3411207","store.size":"2.9gb","pri":"2","rep":"1","creation.date.string":"2026-01-04"},
{"index":"users","health":"green","status":"open","docs.count":"8112","store.size":"24.6mb","pri":"1","rep":"0","creation.date.string":"2024-06-02"},
{"index":".security-7","health":"green","status":"open","docs.count":"132","store.size":"1.2mb","pri":"1","rep":"0","creation.date.string":"2023-11-20"}]`))
		case r.URL.Path == "/_cat/shards" || r.URL.Path == "/_cat/shards/":
			w.Write([]byte(`[{"index":"logs-2026.10.12","shard":0,"prirep":"p","state":"STARTED","docs":4161000,"store":"2.8gb","node":"es-01"},{"index":"logs-2026.10.12","shard":0,"prirep":"r","state":"STARTED","docs":4161000,"store":"2.8gb","node":"es-02"},{"index":"logs-2026.10.12","shard":1,"prirep":"p","state":"STARTED","docs":4162000,"store":"2.8gb","node":"es-02"},{"index":"orders-2026","shard":1,"prirep":"r","state":"UNASSIGNED","node":"","unassigned.reason":"NODE_LEFT"}]`))
		case r.URL.Path == "/_cat/recovery":
			w.Write([]byte(`[{"index":"logs-2026.10.11","shard":"2","type":"store","stage":"done","source":"es-03","node":"es-01","start_time":"2026-10-11T03:00:11.2Z"}]`))
		case r.URL.Path == "/_snapshot":
			w.Write([]byte(`[{"id":"nightly-repo","type":"fs"}]`))
		case r.URL.Path == "/_search" || r.URL.Path == "/_search/":
			w.Write([]byte(`{"took":12,"hits":{"total":{"value":2},"hits":[
{"_index":"logs-2026.10.12","_id":"1","_score":1.0,"_source":{"level":"info","msg":"node started","host":"es-01"}},
{"_index":"logs-2026.10.12","_id":"2","_score":0.8,"_source":{"level":"warn","msg":"disk watermark","host":"es-02"}}]}}`))
		case r.URL.Path == "/logs-2026.10.12/_mapping":
			w.Write([]byte(`{"logs-2026.10.12":{"mappings":{"properties":{"level":{"type":"keyword"},"msg":{"type":"text"},"host":{"type":"keyword"},"@timestamp":{"type":"date"}}}}}`))
		default:
			fmt.Fprint(w, `{}`)
		}
	}))
	defer srv.Close()

	dir := t.TempDir()
	secrets, err := conn.NewFileSecretStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	conns, err := conn.NewStore(dir, secrets)
	if err != nil {
		t.Fatal(err)
	}
	database, err := db.Open(context.Background(), filepath.Join(dir, "smoke.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { database.Close() })

	a := appui.NewApp(ui.NewRouter("/overview"))
	a.AttachDeps(appui.Deps{Conns: conns, DB: database, HomeDir: dir})
	a.SetConnectForm("prod-es-8", srv.URL, "none")

	tt := ui.NewTester(a.View, 1440, 900)
	// 按实机窗口的材质态截（VibrancySidebar）：侧边栏与标题栏透明。
	tt.SetVibrancy(true)
	if err := tt.Click("保存并连接"); err != nil {
		t.Fatalf("click save: %v", err)
	}
	deadline := time.Now().Add(3 * time.Second)
	for !a.Connected && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !a.Connected {
		t.Fatal("not connected")
	}

	shoot := func(name string) {
		t.Helper()
		tt.Frame()
		img := tt.Image()
		if img == nil {
			t.Fatalf("%s: no frame", name)
		}
		f, err := os.Create(filepath.Join(outDir, name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		if err := png.Encode(f, img); err != nil {
			t.Fatal(err)
		}
		t.Logf("wrote %s", name)
	}
	// 等异步数据回填后再取帧（goroutine → update → 下一帧）。
	settle := func() {
		for range 20 {
			tt.Frame()
			time.Sleep(20 * time.Millisecond)
		}
	}
	settle()
	shoot("01-overview")
	for _, nav := range []struct {
		label, name string
	}{
		{"索引", "02-indices"}, {"搜索", "03-search"}, {"控制台", "04-rest"},
		{"节点", "05-nodes"}, {"分片", "06-shards"}, {"快照", "07-snapshots"},
		{"用户", "08-users"}, {"设置", "09-settings"},
	} {
		if err := tt.Click(nav.label); err != nil {
			t.Fatalf("click %s: %v", nav.label, err)
		}
		settle()
		shoot(nav.name)
	}

	// 深色与英文都回到索引页再截（评审要求验证核心表格页的深浅/语言表现）。
	if err := tt.Click("索引"); err != nil {
		t.Fatalf("back to indices: %v", err)
	}
	settle()
	tt.SetDark(true)
	shoot("10-indices-dark")
	tt.SetDark(false)
	a.SetLang("en")
	settle()
	shoot("11-indices-en")
	a.SetLang("zh")
}
