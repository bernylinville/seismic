package overview

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"集群状态一览":           "Cluster at a glance",
		"集群":               "Cluster",
		"未知集群":             "Unknown cluster",
		"健康":               "Health",
		"未知":               "Unknown",
		"活动分片 {0}":         "Active shards {0}",
		"未分配分片 {0}":        "Unassigned shards {0}",
		"状态":               "Status",
		"{0} 主分片 · {1} 副本": "{0} primary · {1} replica shards",
		"数据":               "Data",
		"文档":               "docs",
		"{0} 磁盘占用":         "{0} on disk",
		"{0} 个索引":          "{0} indices",
	})
}
