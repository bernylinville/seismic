package search

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"文档搜索":                       "Document search",
		"索引，如 log-* 或 logs-1,logs-2": "Indices, e.g. log-* or logs-1,logs-2",
		"匹配 {0} 个索引":                 "matches {0} indices",
		"搜索":                         "Search",
		"格式化 JSON":                   "Format JSON",
		"重置":                         "Reset",
		"输入索引 pattern 与查询后点「搜索」": "Enter an index pattern and query, then press Search",
		"搜索中…":             "Searching…",
		"共 {0} 条 · {1} ms": "{0} hits · {1} ms",
		"上一页":              "Previous",
		"下一页":              "Next",
		"没有命中文档":           "No matching documents",
		"格式化失败（第 {0} 行附近）：{1}": "Format failed (near line {0}): {1}",
	})
}
