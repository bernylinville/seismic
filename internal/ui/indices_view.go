package ui

import (
	"context"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

func esClean(name string) string { return es.CleanIndexName(name) }

func indicesCols(lang string) []ui.TableColumn {
	return []ui.TableColumn{
		{Title: kit.Tr(lang, "名称"), Sortable: true},
		{Title: kit.Tr(lang, "健康"), Width: 80, Sortable: true},
		{Title: kit.Tr(lang, "状态"), Width: 70},
		{Title: kit.Tr(lang, "文档数"), Width: 110, Align: ui.End, Sortable: true},
		{Title: kit.Tr(lang, "大小"), Width: 90, Align: ui.End, Sortable: true},
		{Title: kit.Tr(lang, "主分片"), Width: 66, Align: ui.End},
		{Title: kit.Tr(lang, "副本"), Width: 60, Align: ui.End},
		{Title: kit.Tr(lang, "创建时间"), Width: 110},
	}
}

func (a *App) indicesPage(c *ui.Context) {
	s := &a.indices
	if !s.loadedOnce && !s.loading && a.es() != nil {
		a.indicesLoad()
	}
	t := c.Theme()
	// ⌘R 刷新，系统习惯。
	c.OnShortcut(ui.Cmd, ui.KeyR, func() { a.indicesLoad() })

	ui.Column(c).Fill().Children(func() {
		kit.PageHeader(c, kit.Tr(a.lang, "索引"), kit.Tr(a.lang, "{0} 个索引", len(s.visibleRows())), func() {
			ui.Spacer(c)
			ui.SearchField(c, &s.filter).Placeholder(kit.Tr(a.lang, "筛选索引…")).Width(220)
			ui.Checkbox(c, &s.hideSystem, kit.Tr(a.lang, "隐藏系统索引"))
			if ui.Button(c, kit.Tr(a.lang, "刷新")).Clicked() {
				a.indicesLoad()
			}
		})

		if s.actionErr != "" {
			ui.Text(c, s.actionErr).FontSize(kit.SizeCaption(t)).TextColor(t.Danger).Padding(0, t.Space(5), t.Space(1), t.Space(5))
		}
		if kit.StateText(c, a.lang, s.loading, s.err, len(s.rows) == 0, kit.Tr(a.lang, "还没有索引")) {
			return
		}

		rows := s.visibleRows()
		if s.selectedRow >= len(rows) {
			s.selectedRow = -1
		}
		listState := ui.ListState{Selected: &s.selectedRow, Sort: &s.sort, Columns: s.prefs.Frame(a, "indices")}
		table := ui.Table(c, &listState, indicesCols(a.lang), len(rows), func(row, col int) {
			r := rows[row]
			switch col {
			case 0:
				ui.Text(c, r.Index).SingleLine().ContextMenu(func(m *ui.Menu) {
					if m.Item(kit.Tr(a.lang, "查看设置")).Chosen() {
						a.indicesShowDetail(r.Index)
					}
					if m.Item(kit.Tr(a.lang, "刷新")).Chosen() {
						a.indicesAction(kit.Tr(a.lang, "刷新"), r.Index, a.indicesRefreshAction)
					}
					if r.Status == "open" {
						if m.Item(kit.Tr(a.lang, "关闭")).Chosen() {
							a.indicesAction(kit.Tr(a.lang, "关闭"), r.Index, a.indicesCloseAction)
						}
					} else {
						if m.Item(kit.Tr(a.lang, "打开")).Chosen() {
							a.indicesAction(kit.Tr(a.lang, "打开"), r.Index, a.indicesOpenAction)
						}
					}
					m.Separator()
					if m.Item(kit.Tr(a.lang, "删除…")).Chosen() {
						a.indicesAction(kit.Tr(a.lang, "删除"), r.Index, a.indicesDeleteAction)
					}
				})
			case 1:
				ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
					Circle(c, t.Space(1.75), kit.HealthColor(c, r.Health))
					ui.Text(c, r.Health)
				})
			case 2:
				ui.Text(c, r.Status)
			case 3:
				ui.Text(c, r.DocsCount)
			case 4:
				ui.Text(c, r.StoreSize)
			case 5:
				ui.Text(c, r.Pri)
			case 6:
				ui.Text(c, r.Rep)
			case 7:
				ui.Text(c, r.CreationDate).TextColor(t.TextMuted)
			}
		}).Grow(1).Padding(0, t.Space(3), 0, t.Space(3))
		// ui.Table 在拖拽时原地改写 ListState.Columns（列变动没有专门
		// 回调），每帧把改完的布局交回 Sync 比较并持久化。
		s.prefs.Sync(a, "indices", listState.Columns)
		if table.Submitted() {
			a.indicesShowDetail(rows[s.selectedRow].Index)
		}

		if s.detailName != "" {
			a.indicesDetailView(c)
		}
	})
}

func (a *App) indicesDetailView(c *ui.Context) {
	s := &a.indices
	t := c.Theme()
	ui.Column(c).Height(t.Space(55)).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
		Background(t.Surface).Padding(t.Space(3), t.Space(4), t.Space(3), t.Space(4)).Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.detailName).Bold()
			ui.Text(c, kit.Tr(a.lang, "· 设置")).TextColor(t.TextMuted).FontSize(kit.SizeCaption(t))
			ui.Spacer(c)
			if ui.Button(c, kit.Tr(a.lang, "关闭")).Clicked() {
				s.detailName = ""
				s.detail = nil
			}
		})
		if s.detailLoading {
			ui.Text(c, kit.Tr(a.lang, "加载中…")).TextColor(t.TextMuted)
			return
		}
		if len(s.detail) > 0 {
			ui.Text(c, string(s.detail)).Font("monospace").FontSize(kit.SizeMono(t)).Grow(1)
		}
	})
}

// The actions the context menu runs; one client call each, then reload.

func (a *App) indicesRefreshAction(ctx context.Context, client *es.Client, index string) error {
	_, err := client.IndexRefresh(ctx, []string{index})
	return err
}

func (a *App) indicesCloseAction(ctx context.Context, client *es.Client, index string) error {
	_, err := client.IndexClose(ctx, []string{index})
	return err
}

func (a *App) indicesOpenAction(ctx context.Context, client *es.Client, index string) error {
	_, err := client.IndexOpen(ctx, []string{index})
	return err
}

func (a *App) indicesDeleteAction(ctx context.Context, client *es.Client, index string) error {
	_, err := client.IndexDelete(ctx, []string{index})
	return err
}
