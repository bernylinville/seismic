package ui

import (
	"fmt"
	"testing"
)

// benchIndicesState 预填 5k 行（大集群档）。
func benchIndicesState(n int) *IndicesState {
	s := &IndicesState{selectedRow: -1}
	s.rows = make([]IndexRow, n)
	for i := range s.rows {
		s.rows[i] = IndexRow{
			Index:     fmt.Sprintf("perf-idx-%04d", i),
			Health:    []string{"green", "yellow"}[i%2],
			Status:    "open",
			DocsCount: fmt.Sprintf("%d", 1000+i),
			StoreSize: "1kb",
			Pri:        "1",
			Rep:        "0",
		}
	}
	return s
}

// BenchmarkIndicesVisibleRowsCold 每轮换一个过滤词，强制全量重算
// （缓存失效后的最坏情况）。
func BenchmarkIndicesVisibleRowsCold(b *testing.B) {
	s := benchIndicesState(5000)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.filter = fmt.Sprintf("-%d", i%10) // 每轮都变
		s.visibleRows()
	}
}

// BenchmarkIndicesVisibleRowsCached 输入不变：命中缓存（每帧的常态）。
func BenchmarkIndicesVisibleRowsCached(b *testing.B) {
	s := benchIndicesState(5000)
	s.visibleRows() // 预热
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if got := s.visibleRows(); len(got) != 5000 {
			b.Fatalf("rows = %d", len(got))
		}
	}
}
