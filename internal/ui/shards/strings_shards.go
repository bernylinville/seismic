package shards

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"类型":      "Type",
		"原因":      "Reason",
		"{0} 个分片": "{0} shards",
		"还没有分片":   "No shards yet",
		"重新分配…":   "Reroute…",
		"移到节点…":   "Move to node…",
		"主":       "pri",
		"副本":      "repl",
		"重新分配 {0}[{1}]（当前 {2}）到：": "Reroute {0}[{1}] (now on {2}) to:",
		"移动 {0}[{1}] 到：":          "Move {0}[{1}] to:",
		"重新分配失败：{0}":              "Move failed: {0}",
	})
}
