package editor

import (
	"strings"
	"testing"
)

// benchDoc 生成约 n 字节的合法嵌套 JSON。
func benchDoc(n int) string {
	var b strings.Builder
	b.WriteString(`{"hits":[{"_index":"logs-2024","_id":"x","_source":{`)
	for b.Len() < n-4 {
		b.WriteString(`"f":"v",`)
	}
	b.WriteString(`"z":1}}]}`)
	return b.String()
}

// BenchmarkTokenize1MB 对 1 MiB 文档做一次完整 tokenize（高亮路径的
// 单次成本；超大文档在降级模式下会整体跳过）。
func BenchmarkTokenize1MB(b *testing.B) {
	src := benchDoc(1 << 20)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if toks := Tokenize(src); len(toks) == 0 {
			b.Fatal("no tokens")
		}
	}
}

// BenchmarkValidate1MB 对 1 MiB 文档做一次完整 JSON 校验（打开/防抖
// 后的逐键路径单次成本）。
func BenchmarkValidate1MB(b *testing.B) {
	src := benchDoc(1 << 20)
	b.SetBytes(int64(len(src)))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Validate(src); err != nil {
			b.Fatal(err)
		}
	}
}
