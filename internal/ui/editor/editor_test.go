package editor

import (
	"reflect"
	"testing"

	"github.com/egoist/mygo/ui"
)

// ---- tokenizer ----

func TestTokenize(t *testing.T) {
	tests := []struct {
		name string
		src  string
		want []Token
	}{
		{"空文档", "", nil},
		{"空对象", "{}", []Token{
			{TokPunct, 0, 1}, {TokPunct, 1, 2},
		}},
		{"键值与数字", `{"a": 1}`, []Token{
			{TokPunct, 0, 1}, {TokKey, 1, 4}, {TokPunct, 4, 5},
			{TokNumber, 6, 7}, {TokPunct, 7, 8},
		}},
		{"嵌套键逐层识别", `{"query":{"match":{"title":"hi"}}}`, []Token{
			{TokPunct, 0, 1}, {TokKey, 1, 8}, {TokPunct, 8, 9}, {TokPunct, 9, 10},
			{TokKey, 10, 17}, {TokPunct, 17, 18}, {TokPunct, 18, 19},
			{TokKey, 19, 26}, {TokPunct, 26, 27}, {TokString, 27, 31},
			{TokPunct, 31, 32}, {TokPunct, 32, 33}, {TokPunct, 33, 34},
		}},
		{"数组字符串不是键", `["a","b"]`, []Token{
			{TokPunct, 0, 1}, {TokString, 1, 4}, {TokPunct, 4, 5},
			{TokString, 5, 8}, {TokPunct, 8, 9},
		}},
		{"转义字符串整体一个 token", `"a\"b"`, []Token{
			{TokString, 0, 6},
		}},
		{"unicode 字符串", `"你好😀"`, []Token{
			{TokString, 0, 5}, // 引号 + 你 + 好 + 😀(1 rune)
		}},
		{"字面量", `true false null`, []Token{
			{TokLiteral, 0, 4}, {TokLiteral, 5, 10}, {TokLiteral, 11, 15},
		}},
		{"残缺字面量兜底", `{"a": tru`, []Token{
			{TokPunct, 0, 1}, {TokKey, 1, 4}, {TokPunct, 4, 5}, {TokLiteral, 6, 9},
		}},
		{"负数与小数", `[-1, 2.5e3]`, []Token{
			{TokPunct, 0, 1}, {TokNumber, 1, 3}, {TokPunct, 3, 4},
			{TokNumber, 5, 10}, {TokPunct, 10, 11},
		}},
		{"截断字符串到文档末尾", `{"a`, []Token{
			{TokPunct, 0, 1}, {TokString, 1, 3},
		}},
		{"末尾悬空转义不越界", `"a\`, []Token{
			{TokString, 0, 3},
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Tokenize(tt.src)
			if len(tt.want) == 0 && len(got) == 0 {
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Tokenize(%q)\n got %#v\nwant %#v", tt.src, got, tt.want)
			}
		})
	}
}

// ---- FormatJSON ----

func TestFormatJSON(t *testing.T) {
	got, err := FormatJSON(`{"query":{"match":{"title":"你好"}},"size":3,"ok":true,"pi":3.14,"none":null,"tags":["a","b"],"meta":{}}`)
	if err != nil {
		t.Fatal(err)
	}
	want := `{
  "query": {
    "match": {
      "title": "你好"
    }
  },
  "size": 3,
  "ok": true,
  "pi": 3.14,
  "none": null,
  "tags": [
    "a",
    "b"
  ],
  "meta": {}
}`
	if got != want {
		t.Errorf("FormatJSON 顺序/缩进不符\n got:\n%s\nwant:\n%s", got, want)
	}
}

func TestFormatJSONPreservesDetails(t *testing.T) {
	// 数字字面量原样保留，键顺序不重排
	got, err := FormatJSON(`{"b":1.50,"a":2}`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"b\": 1.50,\n  \"a\": 2\n}"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
	// 转义字符串重写为最小转义
	got, err = FormatJSON(`{"k":"a\nb"}`)
	if err != nil {
		t.Fatal(err)
	}
	if want := "{\n  \"k\": \"a\\nb\"\n}"; got != want {
		t.Errorf("got %q want %q", got, want)
	}
}

func TestFormatJSONErrors(t *testing.T) {
	for _, src := range []string{``, `{"a":`, `{} {}`} {
		if _, err := FormatJSON(src); err == nil {
			t.Errorf("FormatJSON(%q) 应报错", src)
		}
	}
}

// ---- Validate ----

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		src      string
		wantErr  bool
		wantPos  Pos
		wantText string
	}{
		{"合法", `{"a": [1, true, null]}`, false, Pos{}, ""},
		{"多行合法", "{\n  \"a\": 1\n}", false, Pos{}, ""},
		{"非法字符", `{"a":}`, true, Pos{1, 7, 6}, "非法字符"},
		{"多行定位", "{\n  \"a\": \n}", true, Pos{3, 2, 11}, "非法字符"},
		{"不完整", `{"a": 1`, true, Pos{1, 8, 7}, "JSON 不完整"},
		{"空文档", `   `, true, Pos{1, 1, 0}, "内容为空"},
		{"多余内容", `{"a":1} x`, true, Pos{1, 9, 8}, "JSON 后有多余内容"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pos, err := Validate(tt.src)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Validate(%q) 应报错", tt.src)
				}
				if got := jsonErrText(err); got != tt.wantText {
					t.Errorf("文案 got %q want %q", got, tt.wantText)
				}
				if pos != tt.wantPos {
					t.Errorf("位置 got %+v want %+v", pos, tt.wantPos)
				}
				return
			}
			if err != nil {
				t.Fatalf("Validate(%q) 不应报错: %v", tt.src, err)
			}
		})
	}
}

// ---- ContextAt / Suggest ----

func TestContextAt(t *testing.T) {
	tests := []struct {
		name      string
		src       string
		caret     int // rune 偏移
		kind      PosKind
		prefix    string
		inString  bool
		path      []string
		fieldName bool
		siblings  []string
	}{
		{"空文档值位置", "", 0, PosValue, "", false, nil, false, nil},
		{"对象开头键位置", "{", 1, PosKey, "", false, nil, false, nil},
		{"键名写到一半(字符串内)", `{"ma`, 4, PosKey, "ma", true, nil, false, nil},
		{"冒号后值位置", `{"size": `, 9, PosValue, "", false, nil, false, nil},
		{"值字面量写到一半", `{"size": t`, 10, PosValue, "t", false, nil, false, []string{"size"}},
		{"嵌套路径", `{"query": {"bool": {mu`, 23, PosKey, "mu", false, []string{"query", "bool"}, false, nil},
		{"数组里值位置", `["a", `, 6, PosValue, "", false, nil, false, nil},
		{"字段名位置", `{"match": {"`, 12, PosKey, "", true, []string{"match"}, true, nil},
		{"逗号后回键位置", `{"a": 1, `, 10, PosKey, "", false, nil, false, []string{"a"}},
		{"值后等待逗号", `{"a": 1`, 8, PosValue, "1", false, nil, false, []string{"a"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := ContextAt(tt.src, tt.caret)
			if ctx.Kind != tt.kind {
				t.Errorf("Kind got %v want %v", ctx.Kind, tt.kind)
			}
			if ctx.Prefix != tt.prefix {
				t.Errorf("Prefix got %q want %q", ctx.Prefix, tt.prefix)
			}
			if ctx.InString != tt.inString {
				t.Errorf("InString got %v want %v", ctx.InString, tt.inString)
			}
			if !reflect.DeepEqual(ctx.Path, tt.path) {
				t.Errorf("Path got %v want %v", ctx.Path, tt.path)
			}
			if ctx.FieldName != tt.fieldName {
				t.Errorf("FieldName got %v want %v", ctx.FieldName, tt.fieldName)
			}
			if len(tt.siblings) > 0 && !reflect.DeepEqual(ctx.Siblings, tt.siblings) {
				t.Errorf("Siblings got %v want %v", ctx.Siblings, tt.siblings)
			}
		})
	}
}

func TestSuggestKeys(t *testing.T) {
	ctx := ContextAt(`{"qu`, 4)
	items := Suggest(&ctx, nil)
	if len(items) != 2 || items[0].Label != "query" || items[1].Label != "query_string" {
		t.Fatalf("prefix=qu 建议应为 query/query_string, got %v", labels(items))
	}
	if items[0].Insert != `query": ` {
		t.Errorf("键插入串 got %q", items[0].Insert)
	}

	// 已存在的兄弟键不再建议
	ctx = ContextAt(`{"size": 1, {"size`, 100) // 非法输入兜不住时也应安全
	ctx = ContextAt(`{"size": {"`, 11)
	ctx.Siblings = []string{"size"}
	for _, it := range Suggest(&ctx, nil) {
		if it.Label == "size" {
			t.Error("兄弟键 size 不应再建议")
		}
	}
}

func TestSuggestValues(t *testing.T) {
	ctx := ContextAt(`{"ok": `, 7)
	items := Suggest(&ctx, nil)
	if got := labels(items); !equal(got, []string{"true", "false", "null", `""`}) {
		t.Errorf("值建议 got %v", got)
	}
	ctx = ContextAt(`{"ok": fa`, 9)
	if got := labels(Suggest(&ctx, nil)); !equal(got, []string{"false"}) {
		t.Errorf("prefix=fa got %v", got)
	}
}

func TestSuggestMappingFields(t *testing.T) {
	mapping := map[string]FieldInfo{
		"views": {Name: "views", Type: "long"},
		"title": {Name: "title", Type: "text"},
	}
	ctx := ContextAt(`{"match": {"`, 12)
	if !ctx.FieldName {
		t.Fatal("应在字段名位置")
	}
	items := Suggest(&ctx, mapping)
	if len(items) != 2 || items[0].Label != "title" || items[0].Detail != "text" {
		t.Fatalf("字段建议按名排序带类型徽标, got %+v", items)
	}
	if items[1].Label != "views" || items[1].Detail != "long" {
		t.Errorf("第二项 got %+v", items[1])
	}
	// 无 mapping 时退回 DSL 词典
	if items := Suggest(&ctx, nil); len(items) == 0 || items[0].Label == "title" {
		t.Error("无 mapping 应退回 DSL 词典")
	}
}

func TestSuggestDocKeys(t *testing.T) {
	ctx := ContextAt(`{"custom_key": 1, {"`, 20)
	ctx.DocKeys = []string{"custom_key"}
	ctx.Kind = PosKey
	found := false
	for _, it := range Suggest(&ctx, nil) {
		if it.Label == "custom_key" && it.Detail == "文档中已出现" {
			found = true
		}
	}
	if !found {
		t.Error("文档中已出现的键应被建议")
	}
}

// ---- 编辑器（不依赖窗口的核心行为） ----

func newTestEditor() *Editor {
	e := New()
	var clip string
	e.ReadClipboard = func() string { return clip }
	e.WriteClipboard = func(s string) { clip = s }
	return e
}

func TestEditorTextRoundtrip(t *testing.T) {
	e := newTestEditor()
	e.SetText(`{"a":1}`)
	if e.Text() != `{"a":1}` {
		t.Fatal(e.Text())
	}
	if e.status.lines != 1 || e.status.chars != 7 {
		t.Errorf("状态 got %d 行 %d 字符", e.status.lines, e.status.chars)
	}
	if e.status.err != nil {
		t.Errorf("合法 JSON 不应报错: %v", e.status.err)
	}
}

func TestEditorTabAndIndent(t *testing.T) {
	e := newTestEditor()
	e.SetText("a")
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 1, End: 1}}
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyTab})
	if e.Text() != "a  " {
		t.Errorf("Tab 应插两空格 got %q", e.Text())
	}
}

func TestEditorEnterAutoIndent(t *testing.T) {
	e := newTestEditor()
	e.SetText("{}")
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 1, End: 1}}
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEnter})
	if got, want := e.Text(), "{\n  \n}"; got != want {
		t.Errorf("括号间回车 got %q want %q", got, want)
	}
	if e.sel.Range.Start != 4 { // 中间行缩进后
		t.Errorf("光标 got %d want 4", e.sel.Range.Start)
	}

	// 前导空白来自光标所在行；括号后加一层
	e.SetText("{\n    \"a\": 1\n}")
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 12, End: 12}} // 行 1 末尾
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEnter})
	if got, want := e.Text(), "{\n    \"a\": 1\n    \n}"; got != want {
		t.Errorf("缩进继承 got %q want %q", got, want)
	}

	// 普通行复制前导空白
	e.SetText("  \"a\": 1")
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 7, End: 7}}
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEnter})
	if got, want := e.Text(), "  \"a\": \n  1"; got != want {
		t.Errorf("普通行回车 got %q want %q", got, want)
	}
}

func TestEditorUndoRedo(t *testing.T) {
	e := newTestEditor()
	e.SetText("")
	// 连续插入合并为一步
	for _, s := range []string{`{`, `"`, `a`} {
		r := ui.TextInputRange{Start: e.buf.UTF16Len(), End: e.buf.UTF16Len()}
		e.ReplaceText(&r, s)
	}
	if e.Text() != `{"a` {
		t.Fatalf("got %q", e.Text())
	}
	e.undo()
	if e.Text() != "" {
		t.Errorf("合并撤销应回到空, got %q", e.Text())
	}
	e.redo()
	if e.Text() != `{"a` {
		t.Errorf("重做 got %q", e.Text())
	}
}

func TestEditorBracketMatch(t *testing.T) {
	e := newTestEditor()
	e.SetText(`{"a": [1, 2]}`)
	// 光标在 [ 之后：rune 6 之后即偏移 7（UTF-16 == rune，全 ASCII）
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 7, End: 7}}
	a, b := e.bracketMatch()
	if a != 6 || b != 11 {
		t.Errorf("括号匹配 got (%d,%d) want (6,11)", a, b)
	}
	// 字符串里的括号不参与
	e.SetText(`{"k": "a[b"}`)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 3, End: 3}}
	if a, b := e.bracketMatch(); a != -1 || b != -1 {
		t.Errorf("字符串内括号不应匹配 got (%d,%d)", a, b)
	}
}

func TestEditorJumpTo(t *testing.T) {
	e := newTestEditor()
	e.SetText("{\n  \"a\": 1,\n  \"b\": 2\n}")
	e.JumpTo(3, 6)
	if line := e.snap().LineAt(e.caretRune()); line != 2 {
		t.Fatalf("行 got %d want 2", line)
	}
	start, _ := e.lineU16(2)
	if e.sel.Range.Start != start+5 {
		t.Errorf("列 got %d want %d", e.sel.Range.Start, start+5)
	}
	// 越界钳制
	e.JumpTo(99, 0)
	if e.caretLine() != e.snap().LineCount()-1 {
		t.Error("行越界应钳到最后一行")
	}
}

func TestEditorCompletionFlow(t *testing.T) {
	e := newTestEditor()
	e.SetMapping(map[string]FieldInfo{
		"title": {Name: "title", Type: "text"},
		"user":  {Name: "user", Type: "keyword"},
	})
	e.SetText(`{"match": {"`)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 12, End: 12}}
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Mods: ui.Ctrl, Key: ui.KeySpace})
	if !e.comp.open {
		t.Fatal("Ctrl+Space 应打开补全")
	}
	if got := labels(e.comp.items); !equal(got, []string{"title", "user"}) {
		t.Fatalf("字段建议 got %v", got)
	}
	// 下移选中第二项后回车插入
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyDown})
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEnter})
	if got, want := e.Text(), `{"match": {"user": `; got != want {
		t.Errorf("插入 got %q want %q", got, want)
	}
	if e.comp.open {
		t.Error("插入后应关闭浮层")
	}
}

func TestEditorCompletionValueAndEscape(t *testing.T) {
	e := newTestEditor()
	e.SetText(`{"ok": `)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 7, End: 7}}
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Mods: ui.Ctrl, Key: ui.KeySpace})
	if !e.comp.open || len(e.comp.items) == 0 {
		t.Fatal("值位置应有建议")
	}
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEscape})
	if e.comp.open {
		t.Error("Esc 应关闭浮层")
	}

	// 键位置（字符串内）补全 DSL
	e.SetText(`{"`)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 2, End: 2}}
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Mods: ui.Ctrl, Key: ui.KeySpace})
	e.handleInput(ui.InputEvent{Kind: ui.InputKeyDown, Key: ui.KeyEnter})
	if got, want := e.Text(), `{"query": `; got != want {
		t.Errorf("DSL 插入 got %q want %q", got, want)
	}
}

func TestEditorClipboardCommands(t *testing.T) {
	e := newTestEditor()
	e.SetText(`{"a":1,"b":2}`)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 1, End: 5}}
	if !e.command("copy") {
		t.Fatal("copy 未被处理")
	}
	if e.Text() != `{"a":1,"b":2}` {
		t.Fatal("copy 不应改动文本")
	}
	if !e.command("paste") {
		t.Fatal("paste 未被处理")
	}
	// 选区被粘贴内容替换
	if e.Text() != `{"a":1,"b":2`[:1]+`"a":`+`1,"b":2}` {
		t.Errorf("paste got %q", e.Text())
	}
}

func TestEditorIMEComposition(t *testing.T) {
	e := newTestEditor()
	e.SetText("你好")
	// 光标在末尾（SetText 归零后手动放到末尾），输入法组合“吗”
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 2, End: 2}}
	e.SetMarkedText(nil, "吗", ui.TextInputRange{Start: 1, End: 1})
	if e.Text() != "你好吗" {
		t.Fatalf("组合插入 got %q", e.Text())
	}
	if e.marked == nil || *e.marked != (ui.TextInputRange{Start: 2, End: 3}) {
		t.Fatalf("marked got %+v", e.marked)
	}
	e.UnmarkText()
	if e.Text() != "你好吗" {
		t.Errorf("Unmark 不应删除文本 got %q", e.Text())
	}
}

// ---- View 冒烟（ui.NewTester） ----

func TestEditorViewSmoke(t *testing.T) {
	e := newTestEditor()
	var changes int
	e.OnChange(func() { changes++ })
	tt := ui.NewTester(func(c *ui.Context) { e.View(c) }, 400, 300)

	if err := tt.Click("JSON 编辑器"); err != nil {
		t.Fatal(err)
	}
	if !tt.Focused("JSON 编辑器") {
		t.Fatal("点击后应聚焦")
	}
	tt.Type(`{"a":1}`)
	if e.Text() != `{"a":1}` {
		t.Fatalf("输入 got %q", e.Text())
	}
	if changes == 0 {
		t.Error("OnChange 未触发")
	}
	if !tt.HasText("1 行 · 7 字符") {
		t.Errorf("状态行应显示行数/字符数, texts=%v", tt.Texts())
	}
	if !tt.HasText("JSON 有效") {
		t.Errorf("应显示 JSON 有效, texts=%v", tt.Texts())
	}

	// 输入坏字符 → 状态行显示行列错误
	tt.Type("x")
	if !tt.HasText("第 1 行") {
		t.Errorf("应显示错误行号, texts=%v", tt.Texts())
	}

	// Ctrl+Space 打开补全，Enter 插入第一项
	e.SetText(`{"`)
	e.sel = ui.TextInputSelection{Range: ui.TextInputRange{Start: 2, End: 2}}
	tt.Frame()
	tt.TypeKey(ui.Ctrl, ui.KeySpace, "")
	if !e.comp.open {
		t.Fatal("Ctrl+Space 应打开补全")
	}
	tt.TypeKey(0, ui.KeyEnter, "")
	if got, want := e.Text(), `{"query": `; got != want {
		t.Errorf("回车插入 got %q want %q", got, want)
	}
}

// ---- 小工具 ----

func labels(items []Suggestion) []string {
	out := make([]string, len(items))
	for i, it := range items {
		out[i] = it.Label
	}
	return out
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
