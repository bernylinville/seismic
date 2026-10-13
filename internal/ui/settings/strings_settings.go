package settings

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"集群设置":                "Cluster settings",
		"未连接集群":               "Not connected",
		"{0} 项设置":             "{0} settings",
		"编辑…":                 "Edit…",
		"还没有集群设置":             "No cluster settings yet",
		"分组":                  "Group",
		"键":                   "Key",
		"值":                   "Value",
		"临时":                  "Temporary",
		"永久":                  "Persistent",
		"修改集群设置":              "Edit cluster setting",
		"当前值：{0}":             "Current value: {0}",
		"新值（JSON 或纯文本）":       "New value (JSON or plain text)",
		"重启后失效":               "Temporary (lost on restart)",
		"应用…":                 "Apply…",
		"会把 {0} 的 {1} 改成 {2}": "Will change the {0} setting {1} to {2}",
		"会影响整个集群，确定改吗？": "This affects the whole cluster. Continue?",
		"返回":          "Back",
		"确认修改":        "Confirm change",
		"新值不是合法 JSON": "New value is not valid JSON",
		"修改失败：{0}":    "Change failed: {0}",
	})
}
