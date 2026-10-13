// 危险操作按钮：主题 Danger 填充的系统式破坏性确认按钮。
package kit

import "github.com/egoist/mygo/ui"

// DangerButton styles a destructive action with the theme's Danger fill,
// as the system's destructive buttons; pair it with a confirming modal.
func DangerButton(c *ui.Context, label string) ui.Element {
	t := c.Theme()
	return ui.Button(c, label).Background(t.Danger).TextColor(t.AccentText)
}
