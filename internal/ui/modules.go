package ui

import (
	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

var _ kit.ModuleHost = (*App)(nil)

// ES returns the connected cluster's client.
func (a *App) ES() *es.Client { return a.client }

// ClusterKey is the connected cluster's UUID.
func (a *App) ClusterKey() string { return a.clusterUUID }

// Version is the connected cluster's version.
func (a *App) Version() string { return a.ClusterVer }

// Distribution is "elasticsearch" or "opensearch".
func (a *App) Distribution() string { return a.DistributionName }

// BuildFlavor is non-empty for special deployments.
func (a *App) BuildFlavor() string { return a.buildFlavor }

// DB is the local SQLite store (nil when it failed to open).
func (a *App) DB() *db.Store { return a.deps.DB }

// SetLang switches the UI language ("zh" or "en").
func (a *App) SetLang(lang string) { a.lang = lang }

// Lang is the UI language ("zh" default).
func (a *App) Lang() string { return a.lang }

// Load satisfies kit.ModuleHost.
func (a *App) Load(markLoading func(), fn func() func()) { a.load(markLoading, fn) }

// modulePage routes to a module's builder. Indices stays in this package as
// the exemplar; the rest each own a package under internal/ui/<module>.
func (a *App) modulePage(c *ui.Context, mod module) {
	h := kit.ModuleHost(a)
	switch mod.Path {
	case "/overview":
		a.overviewMod.View(h, c)
	case "/indices":
		a.indicesPage(c)
	case "/search":
		a.searchMod.View(h, c)
	case "/rest":
		a.restMod.View(h, c)
	case "/nodes":
		a.nodesMod.View(h, c)
	case "/shards":
		a.shardsMod.View(h, c)
	case "/recovery":
		a.recoveryMod.View(h, c)
	case "/snapshots":
		a.snapshotsMod.View(h, c)
	case "/repositories":
		a.repositoriesMod.View(h, c)
	case "/templates":
		a.templatesMod.View(h, c)
	case "/settings":
		a.settingsMod.View(h, c)
	case "/users":
		a.usersMod.View(h, c)
	case "/roles":
		a.rolesMod.View(h, c)
	default:
		ui.Text(c, kit.Tr(a.lang, mod.Label))
	}
}
