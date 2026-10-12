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
		"组合模板":      "Composable templates",
		"没有模板":      "No templates",
		"· 定义 JSON": "· Definition JSON",
		"删除模板":      "Delete template",
		"确定删除模板 {0} 吗？此操作不可撤销。": "Delete template {0}? This cannot be undone.",
		"新建模板":           "New template",
		"索引模式（逗号分隔）":     "Index patterns (comma-separated)",
		"JSON 主体":        "JSON body",
		"名称和索引模式不能为空":    "Name and index patterns are required",
		"JSON 主体无效: {0}": "Invalid JSON body: {0}",
		"新建":             "Create",
		"{0}失败: {1}":     "{0} failed: {1}",
	})
}
