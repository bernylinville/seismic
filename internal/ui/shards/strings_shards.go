package shards

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"类型":      "Type",
		"未分配原因":   "Unassigned reason",
		"{0} 个分片": "{0} shards",
		"没有分片":    "No shards",
		"重新分配…":   "Reroute…",
		"分配到节点…":  "Allocate to node…",
		"主":       "pri",
		"副本":      "repl",
		"重新分配 {0}[{1}]（当前 {2}）到：": "Reroute {0}[{1}] (now on {2}) to:",
		"分配 {0}[{1}] 到：":          "Allocate {0}[{1}] to:",
		"重新分配失败: {0}":             "Reroute failed: {0}",
	})
}
