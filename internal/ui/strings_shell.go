package ui

import "seismic/internal/ui/kit"

// Shell (window chrome, welcome page, connection form) and indices strings.
// Keys are the Chinese source text shown by default; values the English.
func init() {
	kit.RegisterLang(map[string]string{
		// Sidebar groups.
		"集群": "Cluster",
		"数据": "Data",
		"运维": "Ops",
		"安全": "Security",

		// Toolbar, status bar, welcome page.
		"未连接":                      "Not connected",
		"管理连接…":                    "Manage Connections…",
		"刷新健康":                     "Refresh health",
		"工具栏":                      "Toolbar",
		"找不到这个页面":                  "Page not found",
		"{0} 文档":                   "{0} docs",
		"{0} 分片 · {1} 主 · {2} 未分配": "{0} shards · {1} primary · {2} unassigned",
		"跳过证书校验":                   "Skip certificate check",
		"证书是自己签的？勾「跳过证书校验」": "Self-signed certificate? Check “Skip certificate check”",
		"已保存的连接": "Saved connections",
		"连接":     "Connect",

		// Connection form.
		"添加连接":   "Add connection",
		"名称（可选）": "Name (optional)",
		"名称":     "Name",
		"用户名":    "Username",
		"密码":     "Password",
		"可选":     "Optional",
		"测试连接":   "Test connection",
		"保存并连接":  "Save & Connect",
		"测试中…":   "Testing…",
		"无认证":    "No auth",

		// Indices module.
		"文档数":        "Docs",
		"大小":         "Size",
		"主分片":        "Pri",
		"副本":         "Replicas",
		"创建时间":       "Created",
		"{0} 个索引":    "{0} indices",
		"筛选索引…":      "Filter indices…",
		"隐藏系统索引":     "Hide system indices",
		"还没有索引":      "No indices yet",
		"· 设置":       "· Settings",
		"{0} 失败：{1}": "{0} failed: {1}",
	})
}
