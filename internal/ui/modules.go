package ui

import "github.com/egoist/mygo/ui"

// 模块页面：P3 各模块逐个实现，逐文件替换本文件中的对应函数。
// 全部落地后删除本注释与占位实现。

func (a *App) overviewPage(c *ui.Context)     { modulePlaceholder(c, "概览") }
func (a *App) indicesPage(c *ui.Context)      { modulePlaceholder(c, "索引") }
func (a *App) searchPage(c *ui.Context)       { modulePlaceholder(c, "搜索") }
func (a *App) restPage(c *ui.Context)         { modulePlaceholder(c, "REST 控制台") }
func (a *App) nodesPage(c *ui.Context)        { modulePlaceholder(c, "节点") }
func (a *App) shardsPage(c *ui.Context)       { modulePlaceholder(c, "分片") }
func (a *App) recoveryPage(c *ui.Context)     { modulePlaceholder(c, "分片恢复") }
func (a *App) snapshotsPage(c *ui.Context)    { modulePlaceholder(c, "快照") }
func (a *App) repositoriesPage(c *ui.Context) { modulePlaceholder(c, "快照仓库") }
func (a *App) templatesPage(c *ui.Context)    { modulePlaceholder(c, "索引模板") }
func (a *App) settingsPage(c *ui.Context)     { modulePlaceholder(c, "设置") }
func (a *App) usersPage(c *ui.Context)        { modulePlaceholder(c, "用户") }
func (a *App) rolesPage(c *ui.Context)        { modulePlaceholder(c, "角色") }

func modulePlaceholder(c *ui.Context, title string) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(24).Children(func() {
		ui.Text(c, title).FontSize(20).Bold()
		ui.Text(c, "模块实现中").TextColor(t.TextMuted)
	})
}
