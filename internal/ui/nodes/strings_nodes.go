package nodes

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"名称":      "Name",
		"角色":      "Role",
		"堆内存":     "Heap",
		"内存":      "RAM",
		"负载":      "Load",
		"磁盘":      "Disk",
		"角色详情":    "Roles",
		"版本":      "Version",
		"{0} 个节点": "{0} nodes",
		"筛选节点…":   "Filter nodes…",
		"隐藏系统属性":  "Hide system attributes",
		"还没有节点":   "No nodes yet",
		"· 节点详情":  "· Node detail",
	})
}
