// Package ui builds Seismic's window: a vibrancy sidebar with the thirteen
// modules, a toolbar with the cluster selector, and a status bar.
package ui

import (
	"context"
	"fmt"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
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
	{"/rest", "REST 控制台", "data"},
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
	Connected     bool
	ClusterName   string
	ClusterURI    string
	ClusterHealth string // "green" | "yellow" | "red"
	ClusterVer    string
	Distribution  string
	DocsCount     string
	ShardsText    string
	UpdatedText   string

	// win and ctx arrive from main once the window exists; goroutines
	// report back with update, which runs on the main thread.
	win *mygo.Window
	ctx context.Context

	// connect is the add-connection sheet's state.
	connect connectForm

	deps        Deps
	client      *es.Client
	clusterUUID string
	connecting  bool

	// Lang is "zh" or "en".
	Lang string
}

// NewApp makes the app state around the given router.
func NewApp(router *ui.Router) *App {
	return &App{router: router, Lang: "zh", ClusterHealth: "unknown"}
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
	if !a.Connected {
		a.welcome(c)
		return
	}
	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	}
	ui.Column(c).Fill().Children(func() {
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
		health := map[string]string{"green": "● ", "yellow": "● ", "red": "● ", "unknown": "○ "}[a.ClusterHealth]
		name := a.ClusterName
		if name == "" {
			name = "未连接"
		}
		ui.MenuButton(c, health+name, func(m *ui.Menu) {
			if m.Item("管理连接…").Chosen() {
				// 连接管理页在连接模块接线后启用
			}
		})
		ui.Spacer(c)
		lang := "中文"
		if a.Lang != "zh" {
			lang = "English"
		}
		ui.MenuButton(c, lang, func(m *ui.Menu) {
			if m.Item("简体中文").Checked(a.Lang == "zh").Chosen() {
				a.Lang = "zh"
			}
			if m.Item("English").Checked(a.Lang == "en").Chosen() {
				a.Lang = "en"
			}
		})
	}).Label("工具栏")
}

func (a *App) sidebar(c *ui.Context) {
	page := a.router.Path()
	side := ui.Sidebar(c, &page, func() {
		for _, group := range []string{"cluster", "data", "ops", "security"} {
			g := group
			ui.SidebarSection(c, groupTitles[g], nil, func() {
				for _, mod := range modules {
					if mod.Group != g {
						continue
					}
					ui.SidebarItem(c, mod.Path, nil, mod.Label)
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
			r.Title(mod.Label)
			a.modulePage(c, mod)
			return
		}
	}
	ui.Text(c, "页面不存在")
}

// modulePage routes to a module's builder; modules land one file per module.
func (a *App) modulePage(c *ui.Context, mod module) {
	switch mod.Path {
	case "/overview":
		a.overviewPage(c)
	case "/indices":
		a.indicesPage(c)
	case "/search":
		a.searchPage(c)
	case "/rest":
		a.restPage(c)
	case "/nodes":
		a.nodesPage(c)
	case "/shards":
		a.shardsPage(c)
	case "/recovery":
		a.recoveryPage(c)
	case "/snapshots":
		a.snapshotsPage(c)
	case "/repositories":
		a.repositoriesPage(c)
	case "/templates":
		a.templatesPage(c)
	case "/settings":
		a.settingsPage(c)
	case "/users":
		a.usersPage(c)
	case "/roles":
		a.rolesPage(c)
	default:
		ui.Text(c, mod.Label)
	}
}

func (a *App) statusbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(26).Padding(12, 8, 0, 8).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).
		Gap(14).AlignItems(ui.Center).Children(func() {
		Circle(c, 8, healthColor(c, a.ClusterHealth))
		ui.Text(c, a.ClusterName).FontSize(12)
		if a.DocsCount != "" {
			ui.Text(c, fmt.Sprintf("%s 文档", a.DocsCount)).FontSize(12).TextColor(t.TextMuted)
		}
		if a.ShardsText != "" {
			ui.Text(c, a.ShardsText).FontSize(12).TextColor(t.TextMuted)
		}
		ui.Spacer(c)
		if a.ClusterVer != "" {
			ui.Text(c, "v"+a.ClusterVer).FontSize(12).TextColor(t.TextMuted)
		}
		if a.UpdatedText != "" {
			ui.Text(c, a.UpdatedText).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func healthColor(c *ui.Context, health string) ui.Color {
	switch health {
	case "green":
		return ui.Hex("#2f9e63")
	case "yellow":
		return ui.Hex("#d9a406")
	case "red":
		return ui.Hex("#d64545")
	default:
		return c.Theme().TextMuted
	}
}

// Circle draws a small filled dot, the app's one health signal.
func Circle(c *ui.Context, size float32, color ui.Color) ui.Element {
	e := ui.Column(c).Width(size).Height(size).Radius(size / 2).Background(color)
	return e
}

func (a *App) welcome(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Children(func() {
		ui.Column(c).Gap(6).Center().Children(func() {
			ui.Text(c, "Seismic").FontSize(30).Bold()
			ui.Text(c, "macOS 原生 Elasticsearch 客户端").TextColor(t.TextMuted)
		})
		if a.connecting {
			ui.Text(c, "正在连接…").TextColor(t.TextMuted)
		}
		a.connectionsList(c)
		a.connectFormView(c)
	})
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
	ui.Column(c).Gap(4).Width(360).Children(func() {
		ui.Text(c, "已保存的连接").FontSize(12).TextColor(t.TextMuted)
		for _, connItem := range list {
			item := connItem
			ui.Row(c).Gap(8).AlignItems(ui.Center).Padding(8, 6, 8, 6).
				Radius(6).Background(t.Surface).Border(1, t.Border).Children(func() {
				Circle(c, 8, t.TextMuted)
				ui.Column(c).Grow(1).Children(func() {
					ui.Text(c, item.Name)
					ui.Text(c, item.URI).FontSize(11).TextColor(t.TextMuted)
				})
				if ui.Button(c, "连接").Clicked() {
					a.deps.Conns.SetActive(item.UUID)
					a.connectTo(item)
				}
			})
		}
	})
}

func (a *App) connectFormView(c *ui.Context) {
	t := c.Theme()
	f := &a.connect
	ui.Column(c).Gap(8).Width(360).Padding(16).Radius(10).
		Background(t.Surface).Border(1, t.Border).Children(func() {
		ui.Text(c, "添加连接").Bold()
		ui.TextInput(c, &f.Name).Placeholder("名称（可选）").Label("名称")
		ui.TextInput(c, &f.URI).Placeholder("http://localhost:9200").Label("URI")
		ui.MenuButton(c, authLabel(f.AuthType), func(m *ui.Menu) {
			for _, ch := range authChoices {
				choice := ch
				if m.Item(choice.label).Checked(f.AuthType == string(choice.value)).Chosen() {
					f.AuthType = string(choice.value)
				}
			}
		})
		switch es.AuthType(f.AuthType) {
		case es.AuthBasic:
			ui.TextInput(c, &f.Username).Placeholder("elastic").Label("用户名")
			ui.TextInput(c, &f.Password).Placeholder("••••••").Label("密码")
		case es.AuthAPIKey:
			ui.TextInput(c, &f.APIKey).Placeholder("API Key").Label("API Key")
		case es.AuthAWSIAM:
			ui.TextInput(c, &f.AccessKeyID).Label("Access Key ID")
			ui.TextInput(c, &f.SecretAccessKey).Label("Secret Access Key")
			ui.TextInput(c, &f.SessionToken).Placeholder("可选").Label("Session Token")
			ui.TextInput(c, &f.Region).Placeholder("us-east-1").Label("Region")
		}
		ui.Row(c).Gap(8).Children(func() {
			if ui.Button(c, "测试连接").Disabled(f.testing || f.URI == "").Clicked() {
				a.testConnection()
			}
			if ui.PrimaryButton(c, "保存并连接").Disabled(f.URI == "").Clicked() {
				a.saveConnection()
			}
		})
		if f.testing {
			ui.Text(c, "测试中…").TextColor(t.TextMuted)
		}
		if f.tested {
			color := t.TextMuted
			msg := f.testMsg
			if f.testOK {
				color = ui.Hex("#2f9e63")
			} else {
				color = ui.Hex("#d64545")
			}
			ui.Text(c, msg).FontSize(12).TextColor(color)
		}
	})
}
