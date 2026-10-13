package search

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"文档搜索":                       "Document search",
		"在匹配的索引上执行查询":               "Run queries across matching indices",
		"索引，如 log-* 或 logs-1,logs-2": "Indices, e.g. log-* or logs-1,logs-2",
		"匹配 {0} 个索引":                 "matches {0} indices",
		"搜索":                         "Search",
		"格式化 JSON":                   "Format JSON",
		"重置":                         "Reset",
		"填好索引和查询，点「搜索」": "Enter indices and query, then press Search",
		"搜索中…":             "Searching…",
		"共 {0} 条 · {1} ms": "{0} hits · {1} ms",
		"上一页":              "Previous",
		"下一页":              "Next",
		"没搜到任何文档":          "No documents found",
		"格式化失败（第 {0} 行附近）：{1}": "Format failed (near line {0}): {1}",
	})
}
