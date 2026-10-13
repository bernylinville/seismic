package es

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"testing"
	"time"
)

// Perf acceptance against a seeded docker/compose.yml es8 (thousands of
// indices, a wide mapping, 10k hits). Skipped unless SEISMIC_ES8 points at
// the cluster:
//
//	docker compose -f docker/compose.yml up -d
//	go run ./cmd/perfseed -url http://127.0.0.1:9200 -user elastic -pass {{PASSWORD}}
//	SEISMIC_ES8=http://127.0.0.1:9200 SEISMIC_ES8_USER=elastic SEISMIC_ES8_PASS={{PASSWORD}} \
//	  go test ./internal/es/ -run TestPerfAcceptance -v
func perfAuth(t *testing.T) Auth {
	t.Helper()
	if os.Getenv("SEISMIC_ES8") == "" {
		t.Skip("SEISMIC_ES8 not set; see docker/compose.yml + cmd/perfseed")
	}
	if u := os.Getenv("SEISMIC_ES8_USER"); u != "" {
		return Auth{Type: AuthBasic, Username: u, Password: os.Getenv("SEISMIC_ES8_PASS")}
	}
	return Auth{Type: AuthNone}
}

// TestPerfAcceptanceSeededCluster 端到端阈值（真实集群 + perfseed 造数）：
// 5k 索引首屏 < 2s、10k hits 解码 < 3s、宽 mapping 拉取 < 2s。
func TestPerfAcceptanceSeededCluster(t *testing.T) {
	c, err := New(es8URL(t), perfAuth(t))
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	// 5k 索引首屏：cat 解码为表格行。
	start := time.Now()
	var rows []struct {
		Index string `json:"index"`
	}
	if _, err := c.CatIndicesInto(ctx, map[string]any{
		"format": "json", "h": "index",
	}, "perf-idx", &rows); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("5k 索引首屏 %s > 2s", el)
	}
	if len(rows) < 5000 {
		t.Fatalf("seeded indices = %d, want >= 5000 (did perfseed run?)", len(rows))
	}

	// 10k hits：一次搜索全量解码。
	start = time.Now()
	var res benchHitsTarget
	if _, err := c.SearchInto(ctx, map[string]any{"size": 10000}, []string{"perf-hits"}, nil, &res); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el > 3*time.Second {
		t.Fatalf("10k hits 搜索+解码 %s > 3s", el)
	}
	if len(res.Hits.Hits) != 10000 {
		t.Fatalf("hits = %d, want 10000", len(res.Hits.Hits))
	}

	// 宽 mapping：MB 级 JSON 拉取+解码。
	start = time.Now()
	var mapping map[string]mappingReplyShape
	if _, err := c.RequestJSON(ctx, http.MethodGet, "perf-bigmap/_mapping", nil, &mapping); err != nil {
		t.Fatal(err)
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("宽 mapping 拉取+解码 %s > 2s", el)
	}
	if len(mapping) == 0 {
		t.Fatal("empty mapping")
	}
}

// mappingReplyShape 对齐搜索模块的 _mapping 解码形状。
type mappingReplyShape struct {
	Mappings struct {
		Properties map[string]json.RawMessage `json:"properties"`
	} `json:"mappings"`
}
