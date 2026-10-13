package repositories

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"设置":                "Settings",
		"{0} 个仓库":           "{0} repositories",
		"筛选仓库…":             "Filter repositories…",
		"新建仓库…":             "New repository…",
		"未连接集群":             "Not connected",
		"还没有仓库":             "No repositories yet",
		"验证":                "Verify",
		"新建仓库":              "New repository",
		"仓库名称":              "Repository name",
		"设置 JSON":           "Settings JSON",
		"仓库 settings，如 {0}": "Repository settings, e.g. {0}",
		"创建":                "Create",
		"删除仓库":              "Delete repository",
		"删除后快照会失效，确定删 {0}？": "Delete {0}? Its snapshots will stop working.",
		"名称不能为空":            "Name is required",
		"设置 JSON 无效：{0}":    "Invalid settings JSON: {0}",
		"{0} 验证通过":          "{0} verified",
	})
}
