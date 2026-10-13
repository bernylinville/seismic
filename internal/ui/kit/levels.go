package kit

import "github.com/egoist/mygo/ui"

// Level 是跨模块共享的语义状态级别：健康点、分片/恢复/快照状态、
// HTTP 状态码、集群设置分区等统一归到这四级；各模块只负责把自己的
// 域值翻译成 Level，主题色由 LevelColor 单点映射。
type Level int

const (
	LevelNeutral Level = iota // 未知 / 未开始 / 信息性（如 1xx、3xx）
	LevelSuccess
	LevelWarning
	LevelDanger
)

// LevelColor 把 Level 映射到主题语义色，中性级用次级文本色。
func LevelColor(t *ui.Theme, l Level) ui.Color {
	switch l {
	case LevelSuccess:
		return t.Success
	case LevelWarning:
		return t.Warning
	case LevelDanger:
		return t.Danger
	default:
		return t.TextMuted
	}
}
