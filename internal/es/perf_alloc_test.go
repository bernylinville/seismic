package es

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"runtime"
	"testing"
)

// TestRequestJSONAllocBudget 锁住流式解码的分配体量：RequestJSON 每轮
// 总分配 ≤ 5×payload（覆盖解码结构 + json.Decoder 内部缓冲的倍增
// 累计 ≈2×payload，留出余量）。叠加整份 raw 拷贝、多余 string() 转换
// 或 json.Indent 之类的回归会把系数推过界。
//
// 注：TotalAlloc 是累计值，解码器缓冲的倍增长是即时垃圾；本断言锁定
// 的是「没有额外的整份拷贝堆积」，峰值语义见 client_bench_test.go 的
// RequestJSON vs RequestRaw 对照。
func TestRequestJSONAllocBudget(t *testing.T) {
	payload := benchHitsPayload(10000)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write(payload)
	}))
	t.Cleanup(srv.Close)
	client, err := New(srv.URL, Auth{Type: AuthNone})
	if err != nil {
		t.Fatal(err)
	}

	allocs := func(fn func()) uint64 {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		fn()
		runtime.ReadMemStats(&after)
		return after.TotalAlloc - before.TotalAlloc
	}

	const rounds = 3
	var streamTotal uint64
	for i := 0; i < rounds; i++ {
		streamTotal += allocs(func() {
			var out benchHitsTarget
			if _, err := client.RequestJSON(context.Background(), http.MethodGet, "_search", nil, &out); err != nil {
				t.Fatal(err)
			}
		})
	}

	perOp := streamTotal / rounds
	budget := uint64(5 * float64(len(payload)))
	if perOp > budget {
		t.Fatalf("RequestJSON 每轮分配 %d B 超出 5×payload（%d B）的预算", perOp, budget)
	}

	// 对照参考：裸 Unmarshal 的分配（不设硬界，只在日志里给出基线）。
	var unmarshalTotal uint64
	for i := 0; i < rounds; i++ {
		unmarshalTotal += allocs(func() {
			var out benchHitsTarget
			if err := json.Unmarshal(payload, &out); err != nil {
				t.Fatal(err)
			}
		})
	}
	t.Logf("payload=%d B streamed=%d B/op unmarshal=%d B/op", len(payload), perOp, unmarshalTotal/rounds)
}
