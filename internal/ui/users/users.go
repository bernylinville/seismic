// Package users builds the「用户」module page.
package users

import (
	"github.com/egoist/mygo/ui"

	"seismic/internal/ui/kit"
)

// Module holds the 用户 module's state.
type Module struct {
	loadedOnce bool
}

// New makes the module's state.
func New() *Module { return &Module{} }

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(16).Children(func() {
		ui.Text(c, "用户").FontSize(20).Bold()
		ui.Text(c, "模块实现中").TextColor(t.TextMuted)
	})
}
