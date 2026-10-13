// 字号阶梯：对齐 macOS 系统字号（caption 11 / body 13 / 标题 17 / 大数字
// largeTitle 26），由主题 FontSize 相对推导，跟随系统字体与缩放设置。
package kit

import "github.com/egoist/mygo/ui"

// SizeCaption sizes secondary labels and card captions.
func SizeCaption(t *ui.Theme) float32 { return t.Rem(0.85) }

// SizeBody sizes body text, the default reading size.
func SizeBody(t *ui.Theme) float32 { return t.Rem(1) }

// SizeTitle sizes page and card headlines.
func SizeTitle(t *ui.Theme) float32 { return t.Rem(1.3) }

// SizeLarge sizes the page's hero numbers.
func SizeLarge(t *ui.Theme) float32 { return t.Rem(2) }

// SizeMono sizes code and JSON text: a notch under body, as the system's
// consoles do.
func SizeMono(t *ui.Theme) float32 { return t.Rem(0.92) }
