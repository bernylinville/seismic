package ui

import (
	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
)

// es returns the connected cluster's client, nil when not connected.
func (a *App) es() *es.Client { return a.client }

// clusterKey returns the connected cluster's UUID, the partition key for
// per-cluster data (REST history, table prefs); "" when not connected.
func (a *App) clusterKey() string { return a.clusterUUID }

// load runs fn in a goroutine and applies what it returns on the main
// thread: markLoading flips the module's loading flag, fn does the request
// work off-thread and returns the state-apply closure.
func (a *App) load(markLoading func(), fn func() func()) {
	markLoading()
	a.invalidate()
	go func() {
		apply := fn()
		a.update(apply)
	}()
}

// errMessage flattens an error for display, surfacing the ES error body.
func errMessage(err error) string {
	if err == nil {
		return ""
	}
	if httpErr, ok := err.(*es.HTTPError); ok {
		return httpErr.Error()
	}
	return err.Error()
}

// loadingText renders a module's loading/error/empty state row; it returns
// true when the module should not build its content this frame.
func stateText(c *ui.Context, loading bool, err error, empty bool, emptyText string) bool {
	t := c.Theme()
	switch {
	case loading:
		ui.Text(c, "加载中…").TextColor(t.TextMuted)
		return true
	case err != nil:
		ui.Column(c).Grow(1).Padding(16).Children(func() {
			ui.Text(c, "请求失败").Bold().TextColor(ui.Hex("#d64545"))
			ui.Text(c, errMessage(err)).TextColor(t.TextMuted).FontSize(12)
		})
		return true
	case empty:
		ui.Text(c, emptyText).TextColor(t.TextMuted)
		return true
	}
	return false
}
