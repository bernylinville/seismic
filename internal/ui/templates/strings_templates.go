package templates

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"顺序":        "Order",
		"索引模式":      "Index patterns",
		"未连接集群":     "Not connected",
		"{0} 个模板":   "{0} templates",
		"筛选模板…":     "Filter templates…",
		"新建…":       "New…",
		"查看 JSON":   "View JSON",
		"旧版模板":      "Legacy templates",
		"新版模板":      "New templates",
		"还没有模板":     "No templates yet",
		"· 定义 JSON": "· Definition JSON",
		"删除模板":      "Delete template",
		"删了就恢复不了，确定删 {0}？": "Delete {0}? This can't be undone.",
		"新建模板":          "New template",
		"索引模式（逗号分隔）":    "Index patterns (comma-separated)",
		"JSON 内容":       "JSON content",
		"名称和索引模式都要填":    "Name and index patterns are required",
		"JSON 内容无效：{0}": "Invalid JSON content: {0}",
		"新建":            "Create",
		"{0}失败：{1}":     "{0} failed: {1}",
	})
}
