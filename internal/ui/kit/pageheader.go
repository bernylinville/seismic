package kit

import "github.com/egoist/mygo/ui"

// PageHeader 渲染统一的模块页头行：标准边距/间距/垂直居中，左侧
// 「标题 + 说明」列；trailing 非空时在同一行右侧继续构建
// （通常先 ui.Spacer(c) 再放筛选/刷新等控件）。
func PageHeader(c *ui.Context, title, caption string, trailing func()) {
	t := c.Theme()
	ui.Row(c).Padding(t.Space(4), t.Space(5), t.Space(2), t.Space(5)).
		Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Children(func() {
			ui.Text(c, title).FontSize(SizeTitle(t)).Bold()
			ui.Text(c, caption).FontSize(SizeCaption(t)).TextColor(t.TextMuted)
		})
		if trailing != nil {
			trailing()
		}
	})
}
