package repositories

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"设置":                "Settings",
		"{0} 个仓库":           "{0} repositories",
		"筛选仓库…":             "Filter repositories…",
		"新建仓库…":             "New repository…",
		"未连接集群":             "Not connected",
		"没有仓库":              "No repositories",
		"验证":                "Verify",
		"新建快照仓库":            "New snapshot repository",
		"仓库名称":              "Repository name",
		"设置 JSON":           "Settings JSON",
		"仓库 settings，如 {0}": "Repository settings, e.g. {0}",
		"创建":                "Create",
		"删除仓库":              "Delete repository",
		"确定删除仓库 {0}？其快照将不可用。": "Delete repository {0}? Its snapshots become unusable.",
		"名称不能为空":                "Name is required",
		"settings JSON 无效: {0}": "Invalid settings JSON: {0}",
		"{0} 验证通过":              "{0} verified",
	})
}
