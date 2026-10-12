package recovery

import "seismic/internal/ui/kit"

func init() {
	kit.RegisterLang(map[string]string{
		"类型":        "Type",
		"阶段":        "Stage",
		"来源":        "Source",
		"开始时间":      "Started",
		"{0} 条恢复记录": "{0} recoveries",
		"没有恢复记录":    "No recoveries",
	})
}
