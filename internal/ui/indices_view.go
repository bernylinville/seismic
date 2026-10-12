package ui

import (
	"context"
	"fmt"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
)

func esClean(name string) string { return es.CleanIndexName(name) }

var indicesCols = []ui.TableColumn{
	{Title: "名称", Sortable: true},
	{Title: "健康", Width: 80, Sortable: true},
	{Title: "状态", Width: 70},
	{Title: "文档数", Width: 110, Align: ui.End, Sortable: true},
	{Title: "大小", Width: 90, Align: ui.End, Sortable: true},
	{Title: "主分片", Width: 66, Align: ui.End},
	{Title: "副本", Width: 60, Align: ui.End},
	{Title: "创建时间", Width: 110},
}

func (a *App) indicesPage(c *ui.Context) {
	s := &a.indices
	if !s.loadedOnce && !s.loading && a.es() != nil {
		a.indicesLoad()
	}
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 20, 8, 20).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, "索引").FontSize(19).Bold()
				ui.Text(c, fmt.Sprintf("%d 个索引", len(s.visibleRows()))).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			ui.SearchField(c, &s.filter).Placeholder("筛选索引…").Width(220)
			ui.Checkbox(c, &s.hideSystem, "隐藏系统索引")
			if ui.Button(c, "刷新").Clicked() {
				a.indicesLoad()
			}
		})

		if s.actionErr != "" {
			ui.Text(c, s.actionErr).FontSize(12).TextColor(ui.Hex("#d64545")).Padding(0, 20, 4, 20)
		}
		if stateText(c, s.loading, s.err, len(s.rows) == 0, "没有索引") {
			return
		}

		rows := s.visibleRows()
		if s.selectedRow >= len(rows) {
			s.selectedRow = -1
		}
		listState := ui.ListState{Selected: &s.selectedRow, Sort: &s.sort}
		table := ui.Table(c, &listState, indicesCols, len(rows), func(row, col int) {
			r := rows[row]
			switch col {
			case 0:
				ui.Text(c, r.Index).SingleLine().ContextMenu(func(m *ui.Menu) {
					if m.Item("查看设置").Chosen() {
						a.indicesShowDetail(r.Index)
					}
					if m.Item("刷新").Chosen() {
						a.indicesAction("刷新", r.Index, a.indicesRefreshAction)
					}
					if r.Status == "open" {
						if m.Item("关闭").Chosen() {
							a.indicesAction("关闭", r.Index, a.indicesCloseAction)
						}
					} else {
						if m.Item("打开").Chosen() {
							a.indicesAction("打开", r.Index, a.indicesOpenAction)
						}
					}
					m.Separator()
					if m.Item("删除…").Chosen() {
						a.indicesAction("删除", r.Index, a.indicesDeleteAction)
					}
				})
			case 1:
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					Circle(c, 7, healthColor(c, r.Health))
					ui.Text(c, r.Health).FontSize(12)
				})
			case 2:
				ui.Text(c, r.Status).FontSize(12)
			case 3:
				ui.Text(c, r.DocsCount).FontSize(12)
			case 4:
				ui.Text(c, r.StoreSize).FontSize(12)
			case 5:
				ui.Text(c, r.Pri).FontSize(12)
			case 6:
				ui.Text(c, r.Rep).FontSize(12)
			case 7:
				ui.Text(c, r.CreationDate).FontSize(12).TextColor(t.TextMuted)
			}
		}).Grow(1).Padding(0, 12, 0, 12)
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
	ui.Column(c).Height(220).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
		Background(t.Surface).Padding(12, 16, 12, 16).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, s.detailName).Bold()
			ui.Text(c, "· 设置").TextColor(t.TextMuted).FontSize(12)
			ui.Spacer(c)
			if ui.Button(c, "关闭").Clicked() {
				s.detailName = ""
				s.detail = nil
			}
		})
		if s.detailLoading {
			ui.Text(c, "加载中…").TextColor(t.TextMuted)
			return
		}
		if len(s.detail) > 0 {
			ui.Text(c, string(s.detail)).FontSize(12).Grow(1)
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
