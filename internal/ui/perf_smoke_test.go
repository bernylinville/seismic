package ui

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"seismic/internal/es"
)

// perfCatIndicesPayload 生成 n 行 _cat/indices JSON（大集群档 5k 索引）。
func perfCatIndicesPayload(n int) []byte {
	body := make([]byte, 0, n*180)
	body = append(body, '[')
	for i := 0; i < n; i++ {
		if i > 0 {
			body = append(body, ',')
		}
		body = append(body, fmt.Appendf(nil,
			`{"index":"perf-idx-%04d","health":%q,"status":"open","docs.count":"%d","store.size":"1kb","pri":"1","rep":"0","creation.date.string":"2026-10-13T07:00:00Z"}`,
			i, []string{"green", "yellow"}[i%2], 1000+i)...)
	}
	body = append(body, ']')
	return body
}

// TestPerfSmoke5kIndicesFirstScreen 锁阈值：5k 索引解码 + 表格可见行
// （一帧的 2-3 次调用）合计 < 2s。无集群，走本地 httptest，量的是应用
// 层成本；真实集群端到端见 internal/es 的 perf acceptance（env 门控）。
func TestPerfSmoke5kIndicesFirstScreen(t *testing.T) {
	payload := perfCatIndicesPayload(5000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	client, err := es.New(srv.URL, es.Auth{Type: es.AuthNone})
	if err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	var rows []IndexRow
	if _, err := client.CatIndicesInto(context.Background(),
		map[string]any{"format": "json", "h": "index,health,status,docs.count,store.size,pri,rep,creation.date.string"},
		"", &rows); err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5000 {
		t.Fatalf("rows = %d, want 5000", len(rows))
	}
	s := &IndicesState{selectedRow: -1}
	s.rows = rows
	for i := 0; i < 3; i++ { // 一帧内 count+table+empty 的典型调用次数
		if got := s.visibleRows(); len(got) != 5000 {
			t.Fatalf("visibleRows = %d", len(got))
		}
	}
	if el := time.Since(start); el > 2*time.Second {
		t.Fatalf("5k 索引首屏 %s > 2s", el)
	}
}
