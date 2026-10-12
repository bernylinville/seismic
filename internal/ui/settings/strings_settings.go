package settings

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"集群设置":                 "Cluster settings",
		"未连接集群":                "Not connected",
		"{0} 项设置":              "{0} settings",
		"编辑…":                  "Edit…",
		"没有集群设置":               "No cluster settings",
		"区块":                   "Section",
		"键":                    "Key",
		"值":                    "Value",
		"修改集群设置":               "Edit cluster setting",
		"当前值: {0}":             "Current value: {0}",
		"新值（JSON 或纯文本）":        "New value (JSON or plain text)",
		"写入 transient（重启后失效）":  "Write transient (lost on restart)",
		"应用…":                  "Apply…",
		"将把 {0} 设置 {1} 改为 {2}": "Will change the {0} setting {1} to {2}",
		"集群设置变更会影响整个集群，确定继续吗？": "Cluster setting changes affect the whole cluster. Continue?",
		"返回":          "Back",
		"确认修改":        "Confirm change",
		"新值不是合法 JSON": "New value is not valid JSON",
		"修改失败: {0}":   "Change failed: {0}",
	})
}
