package es

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// benchHitsPayload 生成 n 条 hit 的 _search 回复（大集群档 10k hits 的
// 真实体量）。
func benchHitsPayload(n int) []byte {
	body := make([]byte, 0, n*260)
	body = append(body, `{"took":42,"hits":{"total":{"value":`...)
	body = append(body, fmt.Appendf(nil, "%d", n)...)
	body = append(body, `},"hits":[`...)
	for i := 0; i < n; i++ {
		body = append(body, fmt.Appendf(nil,
			`{"_index":"perf-hits","_id":"h%06d","_score":1.5,"_source":{"@timestamp":"2026-10-13T07:06:21Z","level":"info","host":"node-%d","service":"svc-%d","cpu":%.2f,"msg":"hit %d for the 10k-render benchmark"}},`,
			i, i%7, i%13, float64(i%100)/7.0, i)...)
	}
	// 去掉最后一个逗号
	body = body[:len(body)-1]
	body = append(body, `]}}`...)
	return body
}

// benchHitsTarget 是搜索模块解码 _search 回复的目标形状。
type benchHitsTarget struct {
	Hits struct {
		Total struct {
			Value    int    `json:"value"`
			Relation string `json:"relation"`
		} `json:"total"`
		Hits []struct {
			Index  string          `json:"_index"`
			ID     string          `json:"_id"`
			Score  *float64        `json:"_score"`
			Source json.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// BenchmarkRequestJSONDecode10kHits 流式解码 10k hits（搜索热路径）。
func BenchmarkRequestJSONDecode10kHits(b *testing.B) {
	payload := benchHitsPayload(10000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	b.Cleanup(srv.Close)
	c, err := New(srv.URL, Auth{Type: AuthNone})
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var out benchHitsTarget
		if _, err := c.RequestJSON(context.Background(), http.MethodGet, "_search", nil, &out); err != nil {
			b.Fatal(err)
		}
		if len(out.Hits.Hits) != 10000 {
			b.Fatalf("hits = %d", len(out.Hits.Hits))
		}
	}
}

// BenchmarkRequestRaw10kHits 原始字节路径（REST 控制台用），与流式版
// 对比可见省掉的整份 raw 拷贝。
func BenchmarkRequestRaw10kHits(b *testing.B) {
	payload := benchHitsPayload(10000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	b.Cleanup(srv.Close)
	c, err := New(srv.URL, Auth{Type: AuthNone})
	if err != nil {
		b.Fatal(err)
	}
	b.SetBytes(int64(len(payload)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		resp, err := c.Request(context.Background(), http.MethodGet, "_search", nil)
		if err != nil {
			b.Fatal(err)
		}
		if len(resp.Body) != len(payload) {
			b.Fatalf("body = %d", len(resp.Body))
		}
	}
}
