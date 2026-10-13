// Package kit holds what every Seismic module shares: the host interface
// modules get, and the small rendering helpers (health dot, state rows).
package kit

import (
	"context"
	"errors"

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
	// Lang is the UI language ("zh" or "en").
	Lang() string
	// Load runs fn off-thread; markLoading flips a loading flag first and
	// fn returns the closure that applies the result on the main thread.
	// key identifies the logical load: starting a new load with the same
	// key cancels the in-flight one (re-queries don't pile up), and a
	// cluster switch cancels everything. Workers must pass ctx to their
	// ES calls so cancellation aborts the request.
	Load(key string, markLoading func(), fn func(ctx context.Context) func())
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

// HealthColor maps green/yellow/red to the theme's semantic colors, which
// follow the system's light and dark appearance.
func HealthColor(c *ui.Context, health string) ui.Color {
	t := c.Theme()
	switch health {
	case "green":
		return t.Success
	case "yellow":
		return t.Warning
	case "red":
		return t.Danger
	default:
		return t.TextMuted
	}
}

// Dot draws the small filled health circle.
func Dot(c *ui.Context, size float32, color ui.Color) ui.Element {
	return ui.Column(c).Width(size).Height(size).Radius(size / 2).Background(color)
}

// StateText renders a module's loading/error/empty state; true means the
// module should build nothing else this frame.
func StateText(c *ui.Context, lang string, loading bool, err error, empty bool, emptyText string) bool {
	t := c.Theme()
	switch {
	case loading:
		ui.Text(c, Tr(lang, "加载中…")).TextColor(t.TextMuted)
		return true
	case err != nil:
		if errors.Is(err, context.Canceled) {
			ui.Text(c, Tr(lang, "已取消（切换了连接或被新查询取代）")).TextColor(t.TextMuted)
			return true
		}
		ui.Column(c).Grow(1).Padding(t.Space(4)).Children(func() {
			ui.Text(c, Tr(lang, "请求失败")).Bold().TextColor(t.Danger)
			ui.Text(c, ErrMessage(err)).TextColor(t.TextMuted).FontSize(SizeCaption(t))
		})
		return true
	case empty:
		ui.Text(c, emptyText).TextColor(t.TextMuted)
		return true
	}
	return false
}
