// Package kit holds what every Seismic module shares: the host interface
// modules get, and the small rendering helpers (health dot, state rows).
package kit

import (
	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
)

// ModuleHost is what the app gives every module page.
type ModuleHost interface {
	// ES returns the connected cluster's client, nil when not connected.
	ES() *es.Client
	// ClusterKey is the connected cluster's UUID, the per-cluster data key.
	ClusterKey() string
	// Version is the connected cluster's version ("" when unknown).
	Version() string
	// Distribution is "elasticsearch" or "opensearch".
	Distribution() string
	// BuildFlavor is non-empty for special deployments ("serverless").
	BuildFlavor() string
	// DB is the local SQLite store, nil when it failed to open.
	DB() *db.Store
	// Load runs fn off-thread; markLoading flips a loading flag first and
	// fn returns the closure that applies the result on the main thread.
	Load(markLoading func(), fn func() func())
}

// ErrMessage flattens an error for display, surfacing the ES error body.
func ErrMessage(err error) string {
	if err == nil {
		return ""
	}
	if httpErr, ok := err.(*es.HTTPError); ok {
		return httpErr.Error()
	}
	return err.Error()
}

// HealthColor maps green/yellow/red to the app's health colors.
func HealthColor(c *ui.Context, health string) ui.Color {
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

// Dot draws the small filled health circle.
func Dot(c *ui.Context, size float32, color ui.Color) ui.Element {
	return ui.Column(c).Width(size).Height(size).Radius(size / 2).Background(color)
}

// StateText renders a module's loading/error/empty state; true means the
// module should build nothing else this frame.
func StateText(c *ui.Context, loading bool, err error, empty bool, emptyText string) bool {
	t := c.Theme()
	switch {
	case loading:
		ui.Text(c, "加载中…").TextColor(t.TextMuted)
		return true
	case err != nil:
		ui.Column(c).Grow(1).Padding(16).Children(func() {
			ui.Text(c, "请求失败").Bold().TextColor(ui.Hex("#d64545"))
			ui.Text(c, ErrMessage(err)).TextColor(t.TextMuted).FontSize(12)
		})
		return true
	case empty:
		ui.Text(c, emptyText).TextColor(t.TextMuted)
		return true
	}
	return false
}
