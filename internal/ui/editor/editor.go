package editor

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// 布局常量（DIP）。
const (
	monoFamily   = "monospace"
	fontSize     = 13.0
	gutterWidth  = 44.0
	gutterFont   = 11.0
	padX         = 10.0
	padY         = 8.0
	popupRowH    = 22.0
	popupMaxRows = 8
	undoLimit    = 100
)

// palette 从主题派生的 token 配色（暗色可读）。
// 键是任务定死的 teal；字符串用主题 Success 绿，数字/字面量在
// 暗色下换成更亮的变体，结构符号用次级文本色。
type palette struct {
	key, str, num, lit, punct, text, bracket ui.Color
}

func themePalette(t *ui.Theme) palette {
	num := ui.Hex("#3b7dd8")
	lit := ui.Hex("#b0762a")
	if t.Dark {
		num = ui.Hex("#82aaf5")
		lit = ui.Hex("#d9a35c")
	}
	return palette{
		key:     ui.Hex("#0e8c7f"),
		str:     t.Success,
		num:     num,
		lit:     lit,
		punct:   t.TextMuted,
		text:    t.Text,
		bracket: ui.RGBA(127, 140, 141, 0.28),
	}
}

func (p palette) color(t TokenType) ui.Color {
	switch t {
	case TokKey:
		return p.key
	case TokString:
		return p.str
	case TokNumber:
		return p.num
	case TokLiteral:
		return p.lit
	case TokPunct:
		return p.punct
	}
	return p.text
}

type editSnapshot struct {
	text string
	sel  ui.TextInputSelection
}

// Editor 是复用的 JSON 代码编辑器组件：TextInputClient 挂
// ui.TextBuffer，自绘高亮/行号/括号匹配，回车自动缩进，Tab 两空格，
// Ctrl+Space 结构补全。外部用 New 创建、View 挂进页面、
// SetText/Text 读写文本、SetMapping 注入索引字段、JumpTo 跳转。
type Editor struct {
	buf      *ui.TextBuffer
	sel      ui.TextInputSelection
	marked   *ui.TextInputRange
	goalX    float32 // 上下移动保持的目标列（内容坐标）
	dragging bool

	scroll struct{ x, y float32 }

	viewportW, viewportH float32 // 上一帧内容区大小（Draw 时记录）

	tokens        []Token
	tokenVersion  uint64
	lineLayouts   map[int]*ui.TextLayout
	layoutVersion uint64

	bracket struct {
		version uint64
		caret   int
		a, b    int // 匹配括号的 rune 偏移，-1 为无
	}

	mapping map[string]FieldInfo

	comp struct {
		open     bool
		ctx      CompletionContext
		items    []Suggestion
		selected int
		hover    int
		rect     ui.Rect // 元素相对坐标，Draw 时记录
		rows     []ui.Rect
	}

	undoStack []editSnapshot
	redoStack []editSnapshot

	status struct {
		lines, chars int
		err          error
		pos          Pos
	}

	onChange func()

	// 剪贴板策略默认走系统剪贴板；测试里替换为假实现。
	ReadClipboard  func() string
	WriteClipboard func(string)

	pal             palette
	lineHeightCache float32
	undoMergeText   string // 撤销合并链上一步的编辑后文本
}

var _ ui.TextInputClient = (*Editor)(nil)

// New 建一个空编辑器。
func New() *Editor {
	e := &Editor{buf: ui.NewTextBuffer(""), lineLayouts: map[int]*ui.TextLayout{}}
	e.ReadClipboard = mygo.Clipboard.ReadText
	e.WriteClipboard = mygo.Clipboard.WriteText
	e.bracket.a, e.bracket.b = -1, -1
	e.comp.hover = -1
	return e
}

// ---- 对外 API ----

// SetText 整体替换文本（重置撤销栈），光标回到开头。
func (e *Editor) SetText(s string) {
	e.undoStack, e.redoStack, e.undoMergeText = nil, nil, ""
	e.buf.Set(s)
	e.sel = ui.TextInputSelection{}
	e.marked = nil
	e.closeCompletion()
	e.updateStatus()
}

// Text 返回全文。
func (e *Editor) Text() string { return e.buf.String() }

// OnChange 在文本或选区每次变化后回调（主线程）。
func (e *Editor) OnChange(fn func()) { e.onChange = fn }

// SetMapping 注入索引 mapping 的字段（连接切换时由调用方按
// clusterUuid 失效并重注入），字段名补全随即生效。
func (e *Editor) SetMapping(fields map[string]FieldInfo) { e.mapping = fields }

// JumpTo 把光标放到 line 行（1 起）col 列（1 起，≤0 为行首）并滚动
// 到可见；配合 Validate 的行列定位跳到出错处。
func (e *Editor) JumpTo(line, col int) {
	snap := e.snap()
	line = max(1, min(line, snap.LineCount()))
	a, z := snap.LineRange(line - 1)
	runeIdx := a
	if col > 1 {
		runeIdx = min(a+col-1, z)
	}
	u16 := snap.UTF16Offset(runeIdx)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: u16, End: u16}}
	e.closeCompletion()
	e.revealCaret()
}

// ---- TextInputClient 契约（UTF-16 坐标） ----

func (e *Editor) TextForRange(r ui.TextInputRange) (string, ui.TextInputRange) {
	return e.buf.TextForRange(r)
}

func (e *Editor) Selection() ui.TextInputSelection { return e.sel }

func (e *Editor) MarkedRange() (ui.TextInputRange, bool) {
	if e.marked != nil {
		return *e.marked, true
	}
	return ui.TextInputRange{}, false
}

func (e *Editor) ReplaceText(r *ui.TextInputRange, text string) {
	e.replaceUTF16(e.targetRange(r), text)
	e.marked = nil
}

func (e *Editor) SetMarkedText(r *ui.TextInputRange, text string, selected ui.TextInputRange) {
	wanted := e.targetRange(r)
	e.pushUndo()
	actual := e.buf.ReplaceUTF16(wanted, text)
	mark := ui.TextInputRange{Start: actual.Start, End: actual.Start + ui.UTF16Len(text)}
	e.marked = &mark
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{
		Start: mark.Start + selected.Start,
		End:   mark.Start + selected.End,
	}}
	e.afterEdit()
}

func (e *Editor) UnmarkText() { e.marked = nil }

func (e *Editor) BoundsForRange(r ui.TextInputRange) (ui.Rect, ui.TextInputRange, bool) {
	r = e.clampRange(r)
	if r.Start != r.End {
		snap := e.snap()
		line := snap.LineAt(snap.RuneOffset(r.Start))
		if rects := e.lineSelectionRects(line, r); len(rects) > 0 {
			return rects[0], r, true
		}
	}
	return e.caretRectRel(r.Start), r, true
}

func (e *Editor) IndexForPoint(p ui.Point) (int, bool) {
	snap := e.snap()
	lineH := e.lineHeight()
	line := int((p.Y - padY + e.scroll.y) / lineH)
	line = max(0, min(line, snap.LineCount()-1))
	start, end := e.lineU16(line)
	rel := e.layoutFor(line).IndexAt(ui.Point{X: p.X - padX + e.scroll.x, Y: 0})
	return start + max(0, min(rel, end-start)), true
}

// targetRange 解析原生请求的替换范围：nil 表示预编辑，否则选区。
func (e *Editor) targetRange(r *ui.TextInputRange) ui.TextInputRange {
	if r != nil {
		return e.clampRange(*r)
	}
	if e.marked != nil {
		return *e.marked
	}
	return e.sel.Range
}

func (e *Editor) clampRange(r ui.TextInputRange) ui.TextInputRange {
	n := e.buf.UTF16Len()
	r.Start = max(0, min(r.Start, n))
	r.End = max(0, min(r.End, n))
	if r.Start > r.End {
		r.Start, r.End = r.End, r.Start
	}
	return r
}

// ---- 编辑原语（都在主线程；统一走 pushUndo + afterEdit） ----

func (e *Editor) replaceUTF16(r ui.TextInputRange, text string) {
	e.pushUndo()
	actual := e.buf.ReplaceUTF16(r, text)
	end := actual.Start + ui.UTF16Len(text)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: end, End: end}}
	e.afterEdit()
}

// replaceRunes 以 rune 坐标替换（回车缩进/补全插入用），caretRune
// 是插入后的光标 rune 偏移。
func (e *Editor) replaceRunes(r0, r1 int, text string, caretRune int) {
	e.pushUndo()
	e.buf.Replace(r0, r1, text)
	snap := e.snap()
	caretRune = max(0, min(caretRune, snap.Len()))
	u16 := snap.UTF16Offset(caretRune)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: u16, End: u16}}
	e.afterEdit()
}

func (e *Editor) insert(text string) { e.replaceUTF16(e.sel.Range, text) }

func (e *Editor) insertNewline() {
	snap := e.snap()
	r := e.sel.Range
	head := snap.RuneOffset(min(r.Start, r.End))
	tail := snap.RuneOffset(max(r.Start, r.End))
	line := snap.LineAt(head)
	a, z := snap.LineRange(line)
	full := snap.Slice(a, z)
	indent := leadingWhitespace(full)
	var before, after rune
	if head > 0 {
		before, _ = utf8.DecodeLastRuneInString(snap.Slice(head-1, head))
	}
	if tail < snap.Len() {
		after, _ = utf8.DecodeRuneInString(snap.Slice(tail, tail+1))
	}
	text := "\n" + indent
	caret := head + 1 + utf8.RuneCountInString(indent)
	if before == '{' || before == '[' {
		text += "  "
		caret += 2
		if after == '}' || after == ']' {
			text += "\n" + indent
		}
	}
	e.replaceRunes(head, tail, text, caret)
}

func (e *Editor) insertTab() { e.insert("  ") }

// deleteKey 删除选区；无选区时按词或按字素删一格。
func (e *Editor) deleteKey(forward, word bool) {
	r := e.sel.Range
	if r.Start == r.End {
		dir := -1
		if forward {
			dir = 1
		}
		to := e.horizontalTarget(dir, word)
		r = ui.TextInputRange{Start: min(to, r.Start), End: max(to, r.Start)}
		if r.Start == r.End {
			return
		}
	}
	e.replaceUTF16(r, "")
}

func (e *Editor) undo() {
	if len(e.undoStack) == 0 {
		return
	}
	s := e.undoStack[len(e.undoStack)-1]
	e.undoStack = e.undoStack[:len(e.undoStack)-1]
	e.redoStack = append(e.redoStack, editSnapshot{e.buf.String(), e.sel})
	e.restore(s)
}

func (e *Editor) redo() {
	if len(e.redoStack) == 0 {
		return
	}
	s := e.redoStack[len(e.redoStack)-1]
	e.redoStack = e.redoStack[:len(e.redoStack)-1]
	e.undoStack = append(e.undoStack, editSnapshot{e.buf.String(), e.sel})
	e.restore(s)
}

func (e *Editor) restore(s editSnapshot) {
	e.buf.Set(s.text)
	e.sel = ui.TextInputSelection{Range: e.clampRange(s.sel.Range), Reversed: s.sel.Reversed}
	e.marked = nil
	e.closeCompletion()
	e.undoMergeText = s.text
	e.afterEdit()
}

func (e *Editor) pushUndo() {
	// 栈里存每次编辑前的状态。连续单字符编辑（以编辑后文本判定）
	// 合并成一个撤销步：保留最早的编辑前状态，只清重做栈。
	pre := editSnapshot{e.buf.String(), e.sel}
	if len(e.undoStack) > 0 && chainEdit(e.undoMergeText, pre.text) {
		e.undoMergeText = pre.text // 本步编辑前的文本 == 上一步编辑后的文本
		e.redoStack = nil
		return
	}
	e.undoStack = append(e.undoStack, pre)
	if len(e.undoStack) > undoLimit {
		copy(e.undoStack, e.undoStack[1:])
		e.undoStack = e.undoStack[:undoLimit]
	}
	e.undoMergeText = pre.text
	e.redoStack = nil
}

// chainEdit 判断两次编辑后的文本是否只差一个 rune 的追加/删除。
func chainEdit(prevPost, curPost string) bool {
	ra, rb := utf8.RuneCountInString(prevPost), utf8.RuneCountInString(curPost)
	switch rb - ra {
	case 1:
		return strings.HasPrefix(curPost, prevPost)
	case -1:
		return strings.HasPrefix(prevPost, curPost)
	}
	return false
}

func (e *Editor) afterEdit() {
	e.updateStatus()
	if e.comp.open {
		e.refreshCompletion()
	}
	if e.onChange != nil {
		e.onChange()
	}
	e.revealCaret()
}

func (e *Editor) updateStatus() {
	snap := e.snap()
	e.status.lines = snap.LineCount()
	e.status.chars = snap.Len()
	e.status.err, e.status.pos = nil, Pos{}
	if snap.Len() == 0 {
		return
	}
	pos, err := Validate(snap.String())
	e.status.pos, e.status.err = pos, err
}

// ---- 光标移动 ----

func (e *Editor) setCaret(u16 int, extend, keepGoal bool) {
	u16 = max(0, min(u16, e.buf.UTF16Len()))
	if extend {
		anchor := e.sel.Range.Start
		if e.sel.Reversed {
			anchor = e.sel.Range.End
		}
		e.sel = ui.TextInputSelection{
			Range:    ui.TextInputRange{Start: min(anchor, u16), End: max(anchor, u16)},
			Reversed: u16 < anchor,
		}
	} else {
		e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: u16, End: u16}}
	}
	if !keepGoal {
		e.updateGoal()
	}
}

func (e *Editor) updateGoal() {
	snap := e.snap()
	u16 := e.sel.Caret()
	line := snap.LineAt(snap.RuneOffset(u16))
	start, _ := e.lineU16(line)
	e.goalX = e.layoutFor(line).Caret(u16 - start).X
}

// horizontalTarget 返回左右移动（按字素或按词）后的目标偏移。
func (e *Editor) horizontalTarget(dir int, word bool) int {
	u16 := e.sel.Caret()
	if word {
		return e.wordTarget(dir)
	}
	snap := e.snap()
	line := snap.LineAt(snap.RuneOffset(u16))
	start, end := e.lineU16(line)
	layout := e.layoutFor(line)
	if dir < 0 {
		if u16 > start {
			return start + layout.PreviousBoundary(u16-start)
		}
		if line > 0 {
			prevEnd, _ := e.lineU16(line - 1)
			return prevEnd
		}
		return 0
	}
	if u16 < end {
		return start + layout.NextBoundary(u16-start)
	}
	if line+1 < snap.LineCount() {
		nextStart, _ := e.lineU16(line + 1)
		return nextStart
	}
	return e.buf.UTF16Len()
}

// wordTarget 按词移动的目标偏移（词 = 连续字母/数字/下划线）。
func (e *Editor) wordTarget(dir int) int {
	snap := e.snap()
	n := snap.Len()
	cr := snap.RuneOffset(e.sel.Caret())
	isWord := func(i int) bool {
		r, _ := utf8.DecodeRuneInString(snap.Slice(i, i+1))
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	i := cr
	if dir > 0 {
		for i < n && !isWord(i) {
			i++
		}
		for i < n && isWord(i) {
			i++
		}
	} else {
		for i > 0 && !isWord(i-1) {
			i--
		}
		for i > 0 && isWord(i-1) {
			i--
		}
	}
	return snap.UTF16Offset(i)
}

func (e *Editor) moveVertical(dir int, extend bool) {
	snap := e.snap()
	line := snap.LineAt(e.caretRune())
	target := line + dir
	if target < 0 {
		e.setCaret(0, extend, true)
		return
	}
	if target >= snap.LineCount() {
		e.setCaret(e.buf.UTF16Len(), extend, true)
		return
	}
	start, _ := e.lineU16(target)
	rel := e.layoutFor(target).PositionAt(ui.Point{X: e.goalX, Y: 0}).Index
	e.setCaret(start+rel, extend, true)
}

func (e *Editor) selectWordAt(u16 int) {
	snap := e.snap()
	n := snap.Len()
	if n == 0 {
		return
	}
	cr := snap.RuneOffset(u16)
	isWord := func(i int) bool {
		r, _ := utf8.DecodeRuneInString(snap.Slice(i, i+1))
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}
	a, b := cr, cr
	for a > 0 && isWord(a-1) {
		a--
	}
	for b < n && isWord(b) {
		b++
	}
	if a == b {
		return
	}
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{
		Start: snap.UTF16Offset(a),
		End:   snap.UTF16Offset(b),
	}}
	e.updateGoal()
}

func (e *Editor) selectLineAt(u16 int) {
	snap := e.snap()
	line := snap.LineAt(snap.RuneOffset(u16))
	start, end := e.lineU16(line)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: start, End: end}}
	e.updateGoal()
}

// ---- 输入事件 ----

func (e *Editor) handleInput(ev ui.InputEvent) bool {
	switch ev.Kind {
	case ui.InputPointerDown:
		if e.comp.open {
			if pointIn(ev.X, ev.Y, e.comp.rect) {
				for i, row := range e.comp.rows {
					if pointIn(ev.X, ev.Y, row) {
						e.insertCompletion(i)
						return true
					}
				}
				return true // 面板空白处点击不挪光标
			}
			e.closeCompletion()
		}
		to, _ := e.IndexForPoint(ui.Point{X: ev.X, Y: ev.Y})
		switch {
		case ev.Clicks >= 3:
			e.selectLineAt(to)
		case ev.Clicks == 2:
			e.selectWordAt(to)
		default:
			e.setCaret(to, ev.Mods&ui.Shift != 0, false)
			e.dragging = true
		}
		return true
	case ui.InputPointerMove:
		if e.comp.open && ev.Button == -1 {
			hover := -1
			for i, row := range e.comp.rows {
				if pointIn(ev.X, ev.Y, row) {
					hover = i
				}
			}
			if hover != e.comp.hover {
				e.comp.hover = hover
				return true
			}
			return false
		}
		if ev.Button == 0 && e.dragging {
			to, _ := e.IndexForPoint(ui.Point{X: ev.X, Y: ev.Y})
			e.setCaret(to, true, false)
			return true
		}
		return false
	case ui.InputPointerUp:
		if e.dragging {
			e.dragging = false
			return true
		}
		return false
	case ui.InputScroll:
		d := ev.DY
		if !ev.Precise {
			d *= e.lineHeight() * 3
		}
		e.scroll.y = max(0, min(e.scroll.y+d, e.maxScrollY()))
		e.scroll.x = max(0, e.scroll.x+ev.DX)
		return true
	case ui.InputKeyDown:
		return e.keyDown(ev)
	case ui.InputCommand:
		return e.command(ev.Text)
	}
	return false
}

func (e *Editor) keyDown(ev ui.InputEvent) bool {
	if ev.Mods == ui.Ctrl && ev.Key == ui.KeySpace {
		e.toggleCompletion()
		return true
	}
	if e.comp.open {
		switch ev.Key {
		case ui.KeyUp:
			e.moveCompletion(-1)
			return true
		case ui.KeyDown:
			e.moveCompletion(1)
			return true
		case ui.KeyEnter, ui.KeyTab:
			e.insertCompletion(e.comp.selected)
			return true
		case ui.KeyEscape:
			e.closeCompletion()
			return true
		}
	}
	extend := ev.Mods&ui.Shift != 0
	word := ev.Mods&ui.Alt != 0
	switch ev.Key {
	case ui.KeyLeft:
		if ev.Mods&ui.Cmd != 0 {
			e.setCaret(e.lineStartU16(e.caretLine()), extend, false)
		} else {
			e.setCaret(e.horizontalTarget(-1, word), extend, false)
		}
	case ui.KeyRight:
		if ev.Mods&ui.Cmd != 0 {
			_, end := e.lineU16(e.caretLine())
			e.setCaret(end, extend, false)
		} else {
			e.setCaret(e.horizontalTarget(1, word), extend, false)
		}
	case ui.KeyUp:
		if ev.Mods&ui.Cmd != 0 {
			e.setCaret(0, extend, true)
		} else {
			e.moveVertical(-1, extend)
		}
	case ui.KeyDown:
		if ev.Mods&ui.Cmd != 0 {
			e.setCaret(e.buf.UTF16Len(), extend, true)
		} else {
			e.moveVertical(1, extend)
		}
	case ui.KeyHome:
		e.setCaret(e.lineStartU16(e.caretLine()), extend, false)
	case ui.KeyEnd:
		_, end := e.lineU16(e.caretLine())
		e.setCaret(end, extend, false)
	case ui.KeyPageUp:
		e.scrollPage(-1)
	case ui.KeyPageDown:
		e.scrollPage(1)
	case ui.KeyBackspace:
		e.deleteKey(false, ev.Mods&ui.Alt != 0)
	case ui.KeyDelete:
		e.deleteKey(true, ev.Mods&ui.Alt != 0)
	case ui.KeyEnter:
		e.insertNewline()
	case ui.KeyTab:
		e.insertTab()
	default:
		return false
	}
	e.closeCompletion()
	return true
}

func (e *Editor) command(name string) bool {
	switch name {
	case "selectAll":
		n := e.buf.UTF16Len()
		e.sel = ui.TextInputSelection{Range: ui.TextInputRange{End: n}}
		e.updateGoal()
	case "copy":
		if s := e.selectedText(); s != "" {
			e.WriteClipboard(s)
		}
	case "cut":
		s := e.selectedText()
		if s == "" {
			return true
		}
		e.WriteClipboard(s)
		e.replaceUTF16(e.sel.Range, "")
	case "paste":
		if clip := e.ReadClipboard(); clip != "" {
			e.insert(clip)
		}
	case "delete":
		e.replaceUTF16(e.sel.Range, "")
	case "undo":
		e.undo()
	case "redo":
		e.redo()
	default:
		return false
	}
	return true
}

func (e *Editor) selectedText() string {
	r := e.sel.Range
	if r.Start == r.End {
		return ""
	}
	s, _ := e.buf.TextForRange(r)
	return s
}

func (e *Editor) scrollPage(dir int) {
	if e.viewportH <= 0 {
		return
	}
	e.scroll.y = max(0, min(e.scroll.y+float32(dir)*(e.viewportH-e.lineHeight()), e.maxScrollY()))
}

// ---- 补全 ----

func (e *Editor) toggleCompletion() {
	if e.comp.open {
		e.closeCompletion()
		return
	}
	ctx := ContextAt(e.buf.String(), e.caretRune())
	if items := Suggest(&ctx, e.mapping); len(items) > 0 {
		e.comp.open = true
		e.comp.ctx, e.comp.items = ctx, items
		e.comp.selected, e.comp.hover = 0, -1
	}
}

func (e *Editor) closeCompletion() {
	e.comp.open = false
	e.comp.items = nil
	e.comp.hover = -1
}

func (e *Editor) refreshCompletion() {
	if !e.comp.open {
		return
	}
	ctx := ContextAt(e.buf.String(), e.caretRune())
	items := Suggest(&ctx, e.mapping)
	if len(items) == 0 {
		e.closeCompletion()
		return
	}
	e.comp.ctx, e.comp.items = ctx, items
	e.comp.selected = min(e.comp.selected, len(items)-1)
}

func (e *Editor) moveCompletion(d int) {
	e.comp.selected = max(0, min(e.comp.selected+d, len(e.comp.items)-1))
}

func (e *Editor) insertCompletion(i int) {
	if !e.comp.open || i < 0 || i >= len(e.comp.items) {
		return
	}
	s := e.comp.items[i]
	ctx := e.comp.ctx
	cr := e.caretRune()
	r0, r1 := cr, cr
	if ctx.InString {
		r0 = ctx.QuotePos + 1
	} else if ctx.HasWord {
		r0 = ctx.WordPos
	}
	text := s.Insert
	if !ctx.InString && s.Kind == SuggKey {
		text = `"` + text
	}
	caret := r0 + utf8.RuneCountInString(text) - s.CaretBack
	e.comp.open = false
	e.replaceRunes(r0, r1, text, caret)
}

// ---- 视图 ----

// View 构建编辑器：行号列 + 编辑区 + 状态栏。挂在页面的任何容器里，
// 需要 Fill/Grow 的高度。
func (e *Editor) View(c *ui.Context) {
	t := c.Theme()
	e.pal = themePalette(t)
	e.clampScroll()
	e.revealCaret()
	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Key("frame").Grow(1).
			Background(t.Surface).Border(1, t.Border).Radius(t.Radius).Clip().Children(func() {
			ui.Box(c).Key("gutter").Width(gutterWidth).FillHeight().Draw(e.drawGutter)
			ui.Box(c).Key("code").Grow(1).FillHeight().
				Label("JSON 编辑器").Role(ui.RoleTextField).Value(e.buf.String()).
				Cursor(ui.CursorText).
				HandleTextInput(e).HandleInput(e.handleInput).
				Draw(e.drawContent).DrawOver(e.drawPopup)
		})
		ui.Row(c).Key("status").Padding(6, 10).Gap(12).Children(func() {
			ui.Text(c, fmt.Sprintf("%d 行 · %d 字符", e.status.lines, e.status.chars)).
				FontSize(12).TextColor(t.TextMuted)
			switch {
			case e.status.chars == 0:
				ui.Text(c, "空").FontSize(12).TextColor(t.TextMuted)
			case e.status.err != nil:
				ui.Text(c, fmt.Sprintf("第 %d 行 第 %d 列：%s",
					e.status.pos.Line, e.status.pos.Col, jsonErrText(e.status.err))).
					FontSize(12).TextColor(t.Danger)
			default:
				ui.Text(c, "JSON 有效").FontSize(12).TextColor(t.Success)
			}
		})
	})
}

func (e *Editor) drawGutter(p *ui.Painter, r ui.Rect) {
	t := p.Theme()
	lineH := e.lineHeight()
	snap := e.snap()
	first, last := e.visibleLines(snap.LineCount(), lineH)
	caretLine := snap.LineAt(e.caretRune())
	p.Clip(r, 0, func() {
		for line := first; line <= last; line++ {
			s := strconv.Itoa(line + 1)
			color := t.TextMuted
			if line == caretLine {
				color = t.Text
			}
			w, _ := p.MeasureText(0, ui.Span{Text: s, Size: gutterFont})
			y := r.Y + padY + float32(line)*lineH - e.scroll.y
			p.Text(r.X+r.W-8-w, y, s, gutterFont, color)
		}
	})
}

func (e *Editor) drawContent(p *ui.Painter, r ui.Rect) {
	t := p.Theme()
	e.pal = themePalette(t)
	e.viewportW, e.viewportH = r.W, r.H
	lineH := e.lineHeight()
	snap := e.snap()
	first, last := e.visibleLines(snap.LineCount(), lineH)
	p.Clip(r, t.Radius, func() {
		ox := r.X + padX - e.scroll.x
		oy := r.Y + padY - e.scroll.y
		// 选区
		if sel := e.sel.Range; sel.Start != sel.End {
			for line := first; line <= last; line++ {
				for _, b := range e.lineSelectionRects(line, sel) {
					b.X += ox
					b.Y += oy + float32(line)*lineH
					p.Fill(b, t.Selection, 0)
				}
			}
		}
		// 预编辑（输入法组合中）背景
		if e.marked != nil {
			for line := first; line <= last; line++ {
				for _, b := range e.lineSelectionRects(line, *e.marked) {
					b.X += ox
					b.Y += oy + float32(line)*lineH
					p.Fill(ui.Rect{X: b.X, Y: b.Y, W: b.W, H: b.H}, ui.RGBA(127, 140, 141, 0.25), 2)
				}
			}
		}
		// 括号匹配
		if a, b := e.bracketMatch(); a >= 0 {
			for _, pos := range [2]int{a, b} {
				if rect, ok := e.runeRect(pos); ok {
					rect.X += r.X
					rect.Y += r.Y
					p.Fill(rect, e.pal.bracket, 2)
				}
			}
		}
		// 文本（token 着色在 layoutFor 里）
		for line := first; line <= last; line++ {
			p.TextLayout(e.layoutFor(line), ox, oy+float32(line)*lineH, e.pal.text)
		}
		// 光标
		cb := e.caretRectRel(e.sel.Caret())
		cb.X += r.X
		cb.Y += r.Y
		p.Fill(cb, t.Accent, 0)
	})
}

// drawPopup 自绘补全浮层（DrawOver，盖在文本上）。
func (e *Editor) drawPopup(p *ui.Painter, r ui.Rect) {
	if !e.comp.open || len(e.comp.items) == 0 {
		return
	}
	t := p.Theme()
	items := e.comp.items
	top := max(0, min(e.comp.selected-popupMaxRows+1, len(items)-popupMaxRows))
	count := min(popupMaxRows, len(items)-top)
	w := float32(180)
	for _, it := range items[top : top+count] {
		lw, _ := p.MeasureText(0, ui.Span{Text: it.Label, Font: monoFamily, Size: 12})
		dw, _ := p.MeasureText(0, ui.Span{Text: it.Detail, Size: 11})
		w = max(w, lw+dw+48)
	}
	w = min(w, max(180, r.W-8))
	caret := e.caretRectRel(e.sel.Caret())
	h := float32(count)*popupRowH + 8
	x := max(4, min(caret.X, r.W-w-4))
	y := caret.Y + e.lineHeight() + 2
	if y+h > r.H-2 {
		if y2 := caret.Y - h - 2; y2 >= 2 {
			y = y2
		}
	}
	panel := ui.Rect{X: x, Y: y, W: w, H: h}
	e.comp.rect = panel
	e.comp.rows = e.comp.rows[:0]
	p.Clip(r, t.Radius, func() {
		p.Fill(panel, t.Surface, 6)
		p.Stroke(panel, t.Border, 6, 1)
		for i := range count {
			idx := top + i
			row := ui.Rect{X: panel.X + 4, Y: panel.Y + 4 + float32(i)*popupRowH, W: panel.W - 8, H: popupRowH}
			e.comp.rows = append(e.comp.rows, row)
			labelColor, detailColor := e.pal.text, t.TextMuted
			switch {
			case idx == e.comp.selected:
				p.Fill(row, t.Accent, 4)
				labelColor, detailColor = t.AccentText, t.AccentText
			case idx == e.comp.hover:
				p.Fill(row, t.SurfaceHover, 4)
			}
			p.RichText(row.X+8, row.Y+4, row.W-16, ui.Span{
				Text: items[idx].Label, Font: monoFamily, Size: 12, Color: labelColor,
			})
			dw, _ := p.MeasureText(0, ui.Span{Text: items[idx].Detail, Size: 11, Color: detailColor})
			p.RichText(row.X+row.W-8-dw, row.Y+5, dw+2, ui.Span{
				Text: items[idx].Detail, Size: 11, Color: detailColor,
			})
		}
	})
}

// ---- 布局与几何 ----

func (e *Editor) snap() ui.TextSnapshot { return e.buf.Snapshot() }

func (e *Editor) caretRune() int { return e.snap().RuneOffset(e.sel.Caret()) }

func (e *Editor) caretLine() int { return e.snap().LineAt(e.caretRune()) }

func (e *Editor) lineHeight() float32 {
	if e.lineHeightCache == 0 {
		m := (ui.Font{Family: monoFamily, Size: fontSize}).Metrics()
		e.lineHeightCache = m.Ascent + m.Descent + m.LineGap + 2
	}
	return e.lineHeightCache
}

// lineU16 返回行文本的 UTF-16 起止（不含换行）。
func (e *Editor) lineU16(line int) (int, int) {
	snap := e.snap()
	a, z := snap.LineRange(line)
	return snap.UTF16Offset(a), snap.UTF16Offset(z)
}

func (e *Editor) lineStartU16(line int) int {
	start, _ := e.lineU16(line)
	return start
}

func (e *Editor) tokensFor() []Token {
	snap := e.snap()
	if e.tokenVersion != snap.Version() {
		e.tokens = Tokenize(snap.String())
		e.tokenVersion = snap.Version()
	}
	return e.tokens
}

// layoutFor 行布局缓存：编辑后整体失效（版本号），每行按需重建。
func (e *Editor) layoutFor(line int) *ui.TextLayout {
	snap := e.snap()
	if e.layoutVersion != snap.Version() {
		e.lineLayouts = make(map[int]*ui.TextLayout)
		e.layoutVersion = snap.Version()
	}
	if l, ok := e.lineLayouts[line]; ok {
		return l
	}
	a, z := snap.LineRange(line)
	text := snap.Slice(a, z)
	l := ui.ShapeRichText(e.lineSpans(text, a), ui.Font{Family: monoFamily, Size: fontSize}, 0)
	if len(e.lineLayouts) >= 1024 {
		clear(e.lineLayouts)
	}
	e.lineLayouts[line] = l
	return l
}

// lineSpans 把行文本按 token 切成着色 span（同一行内与 token 求交）。
func (e *Editor) lineSpans(text string, lineStart int) []ui.Span {
	if text == "" {
		return nil
	}
	rs := []rune(text)
	toks := e.tokensFor()
	lineEnd := lineStart + len(rs)
	lo := sortSearchToken(toks, lineStart)
	var spans []ui.Span
	pos := 0
	for i := lo; i < len(toks); i++ {
		t := toks[i]
		if t.Start >= lineEnd {
			break
		}
		s := max(t.Start, lineStart) - lineStart
		en := min(t.End, lineEnd) - lineStart
		if s > pos {
			spans = append(spans, ui.Span{Text: string(rs[pos:s]), Color: e.pal.text})
		}
		if en > s {
			spans = append(spans, ui.Span{Text: string(rs[s:en]), Color: e.pal.color(t.Type)})
		}
		pos = max(pos, en)
	}
	if pos < len(rs) {
		spans = append(spans, ui.Span{Text: string(rs[pos:]), Color: e.pal.text})
	}
	return spans
}

// sortSearchToken 找第一个 End > lineStart 的 token（token 按 End 升序）。
func sortSearchToken(toks []Token, lineStart int) int {
	lo, hi := 0, len(toks)
	for lo < hi {
		mid := int(uint(lo+hi) >> 1)
		if toks[mid].End <= lineStart {
			lo = mid + 1
		} else {
			hi = mid
		}
	}
	return lo
}

// visibleLines 返回可见行范围 [first,last]（0 起，含两端）。
func (e *Editor) visibleLines(total int, lh float32) (int, int) {
	first := max(0, int(e.scroll.y/lh))
	rows := 0
	if e.viewportH > 2*padY {
		rows = int((e.viewportH-2*padY)/lh) + 1
	}
	last := min(total-1, first+rows+1)
	return first, max(first, last)
}

func (e *Editor) caretRectRel(u16 int) ui.Rect {
	snap := e.snap()
	u16 = max(0, min(u16, snap.UTF16Len()))
	line := snap.LineAt(snap.RuneOffset(u16))
	start, _ := e.lineU16(line)
	c := e.layoutFor(line).Caret(u16 - start)
	return ui.Rect{
		X: padX + c.X - e.scroll.x,
		Y: padY + float32(line)*e.lineHeight() - e.scroll.y,
		W: 1,
		H: c.H,
	}
}

// lineSelectionRects 返回 u16 范围落在 line 上的矩形（行内坐标）。
func (e *Editor) lineSelectionRects(line int, r ui.TextInputRange) []ui.Rect {
	start, end := e.lineU16(line)
	lo := max(r.Start, start)
	hi := min(r.End, end)
	if lo >= hi {
		return nil
	}
	return e.layoutFor(line).SelectionRects(ui.TextInputRange{Start: lo - start, End: hi - start})
}

// runeRect 返回 rune 所占的矩形（元素相对坐标）。
func (e *Editor) runeRect(pos int) (ui.Rect, bool) {
	snap := e.snap()
	if pos < 0 || pos >= snap.Len() {
		return ui.Rect{}, false
	}
	line := snap.LineAt(pos)
	start, _ := e.lineU16(line)
	rel := snap.UTF16Offset(pos) - start
	rects := e.layoutFor(line).SelectionRects(ui.TextInputRange{Start: rel, End: rel + 1})
	if len(rects) == 0 {
		return ui.Rect{}, false
	}
	b := rects[0]
	b.X += padX - e.scroll.x
	b.Y += padY + float32(line)*e.lineHeight() - e.scroll.y
	return b, true
}

// ---- 括号匹配 ----

// bracketMatch 光标处括号与配对括号的 rune 偏移；无则 -1。只在词法
// 的结构 token 里找，天然跳过字符串里的括号。
func (e *Editor) bracketMatch() (int, int) {
	snap := e.snap()
	cr := e.caretRune()
	if e.bracket.version == snap.Version() && e.bracket.caret == cr {
		return e.bracket.a, e.bracket.b
	}
	e.bracket.version, e.bracket.caret = snap.Version(), cr
	e.bracket.a, e.bracket.b = -1, -1
	var pos = -1
	var open bool
	if cr > 0 {
		if r, _ := utf8.DecodeLastRuneInString(snap.Slice(cr-1, cr)); isBracketRune(r) {
			pos, open = cr-1, isOpenRune(r)
		}
	}
	if pos < 0 && cr < snap.Len() {
		if r, _ := utf8.DecodeRuneInString(snap.Slice(cr, cr+1)); isBracketRune(r) {
			pos, open = cr, isOpenRune(r)
		}
	}
	if pos >= 0 {
		if m := e.scanBracket(pos, open); m >= 0 {
			e.bracket.a, e.bracket.b = min(pos, m), max(pos, m)
		}
	}
	return e.bracket.a, e.bracket.b
}

func (e *Editor) scanBracket(pos int, open bool) int {
	snap := e.snap()
	type pb struct {
		at int
		r  rune
	}
	var list []pb
	for _, t := range e.tokensFor() {
		if t.Type != TokPunct {
			continue
		}
		if r, _ := utf8.DecodeRuneInString(snap.Slice(t.Start, t.Start+1)); isBracketRune(r) {
			list = append(list, pb{t.Start, r})
		}
	}
	idx := -1
	for i := range list {
		if list[i].at == pos {
			idx = i
			break
		}
	}
	if idx < 0 {
		return -1
	}
	want := closingRune(list[idx].r)
	depth := 0
	if open {
		for i := idx + 1; i < len(list); i++ {
			if isOpenRune(list[i].r) {
				depth++
			} else if depth > 0 {
				depth--
			} else if list[i].r == want {
				return list[i].at
			} else {
				return -1
			}
		}
	} else {
		for i := idx - 1; i >= 0; i-- {
			if !isOpenRune(list[i].r) {
				depth++
			} else if depth > 0 {
				depth--
			} else if list[i].r == want {
				return list[i].at
			} else {
				return -1
			}
		}
	}
	return -1
}

func isBracketRune(r rune) bool {
	return strings.ContainsRune("{}[]()", r)
}

func isOpenRune(r rune) bool { return strings.ContainsRune("{[(", r) }

func closingRune(r rune) rune {
	switch r {
	case '{':
		return '}'
	case '[':
		return ']'
	default:
		return ')'
	}
}

// ---- 滚动 ----

func (e *Editor) maxScrollY() float32 {
	if e.viewportH <= 0 {
		return 0
	}
	h := float32(e.buf.LineCount())*e.lineHeight() + 2*padY
	return max(0, h-e.viewportH)
}

func (e *Editor) clampScroll() {
	e.scroll.x = max(0, e.scroll.x)
	e.scroll.y = max(0, min(e.scroll.y, e.maxScrollY()))
}

// revealCaret 把光标行滚进可见区（viewport 尚未知时跳过）。
func (e *Editor) revealCaret() {
	if e.viewportH <= 0 {
		return
	}
	lineH := e.lineHeight()
	line := float32(e.caretLine())
	top := line*lineH - e.scroll.y
	view := e.viewportH - 2*padY
	if top < 0 {
		e.scroll.y = line * lineH
	} else if top+lineH > view {
		e.scroll.y = line*lineH + lineH - view
	}
	e.scroll.y = max(0, min(e.scroll.y, e.maxScrollY()))
}

// ---- 小工具 ----

func leadingWhitespace(s string) string {
	for i, r := range s {
		if r != ' ' && r != '\t' {
			return s[:i]
		}
	}
	return s
}

func pointIn(x, y float32, r ui.Rect) bool {
	return x >= r.X && x < r.X+r.W && y >= r.Y && y < r.Y+r.H
}
