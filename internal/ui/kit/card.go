package kit

import "github.com/egoist/mygo/ui"

// Card 是统一的分组卡片材质：Surface 填充 + 圆角 + 1px 边（系统设置
// 分组卡片观感）。Padding/Gap/Grow 由调用方按版面在链上配置。
func Card(c *ui.Context) ui.Element {
	t := c.Theme()
	return ui.Column(c).Background(t.Surface).Radius(t.Space(3)).
		Border(1, t.Border)
}
