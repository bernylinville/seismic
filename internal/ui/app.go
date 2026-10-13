// Package ui builds Seismic's window: a vibrancy sidebar with the thirteen
// modules, a toolbar with the cluster selector, and a status bar.
package ui

import (
	"context"
	"strings"
	"sync"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
	"seismic/internal/ui/nodes"
	"seismic/internal/ui/overview"
	"seismic/internal/ui/recovery"
	"seismic/internal/ui/repositories"
	"seismic/internal/ui/rest"
	"seismic/internal/ui/roles"
	"seismic/internal/ui/search"
	"seismic/internal/ui/settings"
	"seismic/internal/ui/shards"
	"seismic/internal/ui/snapshots"
	"seismic/internal/ui/templates"
	"seismic/internal/ui/users"
)

type module struct {
	Path  string
	Label string
	Group string
}

// The thirteen modules of elasticvue, in the four sidebar groups.
var modules = []module{
	{"/overview", "概览", "cluster"},
	{"/nodes", "节点", "cluster"},
	{"/shards", "分片", "cluster"},
	{"/recovery", "分片恢复", "cluster"},
	{"/indices", "索引", "data"},
	{"/search", "搜索", "data"},
	{"/rest", "控制台", "data"},
	{"/templates", "索引模板", "data"},
	{"/snapshots", "快照", "ops"},
	{"/repositories", "快照仓库", "ops"},
	{"/settings", "设置", "ops"},
	{"/users", "用户", "security"},
	{"/roles", "角色", "security"},
}

var groupTitles = map[string]string{
	"cluster":  "集群",
	"data":     "数据",
	"ops":      "运维",
	"security": "安全",
}

// App is the whole window's state.
type App struct {
	router *ui.Router

	// Connected cluster, refreshed by the connection flow.
	Connected        bool
	ClusterName      string
	ClusterURI       string
	ClusterHealth    string // "green" | "yellow" | "red"
	ClusterVer       string
	DistributionName string
	distribution     string
	DocsCount        string
	ShardsText       string
	StoreText        string
	UpdatedText      string
	buildFlavor      string
	// win and ctx arrive from main once the window exists; goroutines
	// report back with update, which runs on the main thread.
	win *mygo.Window
	ctx context.Context

	// connect is the add-connection sheet's state.
	connect connectForm

	// delConn* 是删除已存连接的确认框状态。
	delConnOpen bool
	delConnUUID string
	delConnName string

	deps        Deps
	client      *es.Client
	// loads 跟踪在途模块加载的取消句柄（loadsMu 保护；见 support.go load）。
	loadsMu sync.Mutex
	loads   map[string]*loadSlot
	clusterUUID string
	connecting  bool

	// 模块状态：每个模块一个文件（indices_state.go 等）。
	indices IndicesState

	overviewMod     *overview.Module
	searchMod       *search.Module
	restMod         *rest.Module
	nodesMod        *nodes.Module
	shardsMod       *shards.Module
	recoveryMod     *recovery.Module
	snapshotsMod    *snapshots.Module
	repositoriesMod *repositories.Module
	templatesMod    *templates.Module
	settingsMod     *settings.Module
	usersMod        *users.Module
	rolesMod        *roles.Module
	// lang is the UI language, "zh" (default) or "en".
	lang string
}

// NewApp makes the app state around the given router.
func NewApp(router *ui.Router) *App {
	return &App{
		router: router, lang: "zh", ClusterHealth: "unknown",
		overviewMod:     overview.New(),
		searchMod:       search.New(),
		restMod:         rest.New(),
		nodesMod:        nodes.New(),
		shardsMod:       shards.New(),
		recoveryMod:     recovery.New(),
		snapshotsMod:    snapshots.New(),
		repositoriesMod: repositories.New(),
		templatesMod:    templates.New(),
		settingsMod:     settings.New(),
		usersMod:        users.New(),
		rolesMod:        roles.New(),
	}
}

// Attach wires the window (for cross-goroutine updates) and the app context.
func (a *App) Attach(win *mygo.Window, ctx context.Context) {
	a.win = win
	a.ctx = ctx
}

// update runs f on the main thread and draws a frame; background work ends
// with it, as the view reads state on the main thread only. Before Attach
// (tests), it applies f directly.
func (a *App) update(f func()) {
	if a.win != nil {
		a.win.Update(f)
		return
	}
	f()
}

func (a *App) invalidate() {
	if a.win != nil {
		a.win.Invalidate()
	}
}

// View builds the window: a welcome page until a cluster is connected,
// then toolbar, sidebar and routed module pages, and the status bar.
func (a *App) View(c *ui.Context) {
	// 全窗口坐在材质上：侧边栏与标题栏透出毛玻璃，内容列自绘不透明背景。
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	if !a.Connected {
		a.welcome(c)
		return
	}
	ui.Column(c).Fill().Transition(pageEnter).Children(func() {
		a.toolbar(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c)
			ui.Column(c).Grow(1).Background(c.Theme().Background).Children(func() {
				a.router.View(c, func(r *ui.Route) { a.page(c, r) })
			})
		})
		a.statusbar(c)
	})
}

// Router exposes the app's router (for tests and deep links).
func (a *App) Router() *ui.Router { return a.router }

// backgroundCtx returns the app context, or Background before Attach.
func (a *App) backgroundCtx() context.Context {
	if a.ctx != nil {
		return a.ctx
	}
	return context.Background()
}

func (a *App) toolbar(c *ui.Context) {
	ui.Toolbar(c, func() {
		ui.BackButton(c, a.router)
		ui.ForwardButton(c, a.router)
		// 方向 A 的结构性决定：集群选择器居工具栏中央，全局上下文一眼可见。
		ui.Spacer(c)
		health := map[string]string{"green": "● ", "yellow": "● ", "red": "● ", "unknown": "○ "}[a.ClusterHealth]
		name := a.ClusterName
		if name == "" {
			name = kit.Tr(a.lang, "未连接")
		}
		ui.MenuButton(c, health+name, func(m *ui.Menu) {
			if a.deps.Conns != nil {
				for _, item := range a.deps.Conns.List() {
					connItem := item
					if m.Item(connItem.Name).Checked(connItem.UUID == a.deps.Conns.Active()).Chosen() {
						a.deps.Conns.SetActive(connItem.UUID)
						a.connectTo(connItem)
					}
				}
			}
			m.Separator()
			if m.Item(kit.Tr(a.lang, "管理连接…")).Chosen() {
				a.Connected = false
			}
			if m.Item(kit.Tr(a.lang, "刷新健康")).Chosen() {
				a.refreshHealth()
			}
		})
		ui.Spacer(c)
		lang := "中文"
		if a.lang != "zh" {
			lang = "English"
		}
		ui.MenuButton(c, lang, func(m *ui.Menu) {
			if m.Item("简体中文").Checked(a.lang == "zh").Chosen() {
				a.lang = "zh"
			}
			if m.Item("English").Checked(a.lang == "en").Chosen() {
				a.lang = "en"
			}
		})
	}).Label(kit.Tr(a.lang, "工具栏"))
}

func (a *App) sidebar(c *ui.Context) {
	page := a.router.Path()
	side := ui.Sidebar(c, &page, func() {
		for _, group := range []string{"cluster", "data", "ops", "security"} {
			g := group
			ui.SidebarSection(c, kit.Tr(a.lang, groupTitles[g]), nil, func() {
				for _, mod := range modules {
					if mod.Group != g {
						continue
					}
					ui.SidebarItem(c, mod.Path, nil, kit.Tr(a.lang, mod.Label))
				}
			})
		}
	}).Width(220)
	if !c.Vibrancy() {
		side.Background(c.Theme().Surface)
	}
	if side.Changed() {
		a.router.Push(page)
	}
}

func (a *App) page(c *ui.Context, r *ui.Route) {
	for _, mod := range modules {
		if r.Match(mod.Path) {
			r.Title(kit.Tr(a.lang, mod.Label))
			// 模块切换：新页面上浮淡入；系统「减少动态效果」时 mygo 自动停用位移。
			ui.Column(c).Key(mod.Path).Fill().Transition(pageEnter).Children(func() {
				a.modulePage(c, mod)
			})
			return
		}
	}
	ui.Text(c, kit.Tr(a.lang, "找不到这个页面"))
}

// pageEnter 是页面进场过渡：8px 上浮 + 淡入，默认 200ms EaseOut；
// 页面 Key 稳定，同页刷新不会重放。
var pageEnter = ui.ElementTransition{Enter: &ui.Motion{Y: 8}}

func (a *App) statusbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(t.Space(6.5)).Padding(t.Space(3), t.Space(2), 0, t.Space(2)).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
		Gap(t.Space(3.5)).AlignItems(ui.Center).Children(func() {
		Circle(c, t.Space(2), kit.HealthColor(c, a.ClusterHealth))
		ui.Text(c, a.ClusterName).FontSize(kit.SizeCaption(t))
		if a.DocsCount != "" {
			ui.Text(c, kit.Tr(a.lang, "{0} 文档", a.DocsCount)).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
		}
		if a.ShardsText != "" {
			ui.Text(c, a.ShardsText).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
		}
		if a.StoreText != "" {
			ui.Text(c, a.StoreText).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
		}
		ui.Spacer(c)
		if a.ClusterVer != "" {
			ui.Text(c, "v"+a.ClusterVer).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
		}
		if a.UpdatedText != "" {
			ui.Text(c, a.UpdatedText).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
		}
	})
}

// Circle draws a small filled dot, the app's one health signal.
func Circle(c *ui.Context, size float32, color ui.Color) ui.Element {
	e := ui.Column(c).Width(size).Height(size).Radius(size / 2).Background(color)
	return e
}

func (a *App) welcome(c *ui.Context) {
	t := c.Theme()
	// ⌘↩ 保存并连接：表单的默认动作，系统习惯。
	c.OnShortcut(ui.Cmd, ui.KeyEnter, func() {
		if !a.connect.testing && a.connect.URI != "" {
			a.saveConnection()
		}
	})
	ui.Column(c).Fill().Center().Children(func() {
		ui.Column(c).Gap(t.Space(1.5)).Center().Children(func() {
			ui.Text(c, "Seismic").FontSize(kit.SizeLarge(t)).Bold()
			ui.Text(c, kit.Tr(a.lang, "macOS 原生 Elasticsearch 客户端")).TextColor(t.TextMuted)
		})
		if a.connecting {
			ui.Text(c, kit.Tr(a.lang, "正在连接…")).TextColor(t.TextMuted)
		}
		a.connectionsList(c)
		a.connectFormView(c)
	})

	a.deleteConnDialog(c)
}

func (a *App) connectionsList(c *ui.Context) {
	if a.deps.Conns == nil {
		return
	}
	t := c.Theme()
	list := a.deps.Conns.List()
	if len(list) == 0 {
		return
	}
	ui.Column(c).Gap(t.Space(1)).Width(360).Children(func() {
		ui.Text(c, kit.Tr(a.lang, "已保存的连接")).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
		for _, connItem := range list {
			item := connItem
			ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Padding(t.Space(2), t.Space(1.5), t.Space(2), t.Space(1.5)).
				Radius(t.Radius).Background(t.Surface).Border(1, t.Border).ContextMenu(func(menu *ui.Menu) {
				if menu.Item(kit.Tr(a.lang, "连接")).Chosen() {
					a.deps.Conns.SetActive(item.UUID)
					a.connectTo(item)
				}
				menu.Separator()
				if menu.Item(kit.Tr(a.lang, "删除…")).Chosen() {
					a.delConnUUID, a.delConnName, a.delConnOpen = item.UUID, item.Name, true
				}
			}).Children(func() {
				Circle(c, t.Space(2), t.TextMuted)
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, item.Name)
					ui.Text(c, item.URI).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
				})
				if ui.Button(c, kit.Tr(a.lang, "连接")).Clicked() {
					a.deps.Conns.SetActive(item.UUID)
					a.connectTo(item)
				}
			})
		}
	})
}

// deleteConnDialog asks before removing a saved connection.
func (a *App) deleteConnDialog(c *ui.Context) {
	if !a.delConnOpen || a.delConnUUID == "" {
		return
	}
	t := c.Theme()
	ui.Modal(c, &a.delConnOpen, func() {
		ui.Column(c).Width(380).Padding(t.Space(5)).Gap(t.Space(3)).Children(func() {
			ui.Text(c, kit.Tr(a.lang, "删除连接")).FontSize(kit.SizeTitle(t)).Bold()
			ui.Text(c, kit.Tr(a.lang, "删了就恢复不了，确定删 {0}？", a.delConnName)).TextColor(t.TextMuted)
			ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
				if ui.Button(c, kit.Tr(a.lang, "取消")).Clicked() {
					a.delConnOpen = false
				}
				if kit.DangerButton(c, kit.Tr(a.lang, "删除")).Clicked() {
					uuid := a.delConnUUID
					a.delConnOpen = false
					if a.deps.Conns != nil {
						_ = a.deps.Conns.Delete(uuid)
					}
				}
			})
		})
	})
}

func (a *App) connectFormView(c *ui.Context) {
	t := c.Theme()
	f := &a.connect
	ui.Column(c).Gap(t.Space(2)).Width(360).Padding(t.Space(4)).Radius(t.Space(3)).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, kit.Tr(a.lang, "添加连接")).Bold()
		ui.TextInput(c, &f.Name).Placeholder(kit.Tr(a.lang, "名称（可选）")).Label(kit.Tr(a.lang, "名称"))
		ui.TextInput(c, &f.URI).Placeholder("http://localhost:9200").Label("URI")
		ui.MenuButton(c, authLabel(f.AuthType, a.lang), func(m *ui.Menu) {
			for _, ch := range authChoices {
				choice := ch
				if m.Item(choice.label).Checked(f.AuthType == string(choice.value)).Chosen() {
					f.AuthType = string(choice.value)
				}
			}
		})
		switch es.AuthType(f.AuthType) {
		case es.AuthBasic:
			ui.TextInput(c, &f.Username).Placeholder("elastic").Label(kit.Tr(a.lang, "用户名"))
			ui.TextInput(c, &f.Password).Placeholder("••••••").Label(kit.Tr(a.lang, "密码"))
		case es.AuthAPIKey:
			ui.TextInput(c, &f.APIKey).Placeholder("API Key").Label("API Key")
		case es.AuthAWSIAM:
			ui.TextInput(c, &f.AccessKeyID).Label("Access Key ID")
			ui.TextInput(c, &f.SecretAccessKey).Label("Secret Access Key")
			ui.TextInput(c, &f.SessionToken).Placeholder(kit.Tr(a.lang, "可选")).Label("Session Token")
			ui.TextInput(c, &f.Region).Placeholder("us-east-1").Label("Region")
		}
		ui.Checkbox(c, &f.TLSInsecure, kit.Tr(a.lang, "跳过证书校验"))
		ui.Row(c).Gap(t.Space(2)).Children(func() {
			if ui.Button(c, kit.Tr(a.lang, "测试连接")).Disabled(f.testing || f.URI == "").Clicked() {
				a.testConnection()
			}
			if ui.PrimaryButton(c, kit.Tr(a.lang, "保存并连接")).Disabled(f.URI == "").Clicked() {
				a.saveConnection()
			}
		})
		if f.testing {
			ui.Text(c, kit.Tr(a.lang, "测试中…")).TextColor(t.TextMuted)
		}
		if f.tested {
			color := t.TextMuted
			msg := f.testMsg
			if f.testOK {
				color = t.Success
			} else {
				color = t.Danger
				if strings.Contains(strings.ToLower(msg), "x509") || strings.Contains(strings.ToLower(msg), "certificate") {
					msg += "\n" + kit.Tr(a.lang, "证书是自己签的？勾「跳过证书校验」")
				}
			}
			ui.Text(c, msg).FontSize(kit.SizeCaption(t)).TextColor(color)
		}
	})
}
