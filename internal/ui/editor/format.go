package editor

import (
	"encoding/json"
	"errors"
	"io"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Pos 是文档里的一个位置：Line/Col 以 1 计（列按 rune 数），
// Offset 是 rune 偏移，可直接换算为编辑器光标。
type Pos struct {
	Line   int
	Col    int
	Offset int
}

// FormatJSON 重新序列化 src 为两空格缩进的美化 JSON。
// 用 json.Decoder 的 token 流重建，保留对象键的书写顺序
// （json.Marshal map 会按字典序重排，不可用）；数字字面量原样保留
// （UseNumber），字符串按最小转义重写（合法转义与 CJK 原文保留）。
// 空对象/数组输出为内联的 {} 与 []。
// 注意 Token() 不返回冒号和逗号，分隔符由状态机合成。
func FormatJSON(src string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(src))
	dec.UseNumber()
	var b strings.Builder
	depth := 0
	var inObj []bool // 每层是否对象
	first := true    // 顶层首个 token 前不缩进
	pendingColon := false
	needComma := false
	expectKey := false
	skipClose := false // 内联收尾的容器，其闭合符 token 到达时跳过
	done := false      // 顶层值已完成（之后再有 token 即多余内容）
	sep := func() {
		if pendingColon {
			b.WriteString(": ")
			pendingColon = false
			return
		}
		if needComma {
			b.WriteByte(',')
		}
		if first {
			first = false
			return
		}
		b.WriteString(indentTo(depth))
	}
	scalarDone := func() {
		needComma = true
		expectKey = len(inObj) > 0 && inObj[len(inObj)-1]
		if depth == 0 {
			done = true
		}
	}
	for {
		tok, err := dec.Token()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return "", err
		}
		if done {
			return "", errors.New("JSON 后有多余内容")
		}
		if skipClose {
			// 到达的一定是刚内联写掉的那个闭合符
			skipClose = false
			continue
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				sep()
				b.WriteRune(rune(t))
				needComma = false
				if dec.More() {
					depth++
					inObj = append(inObj, t == '{')
					expectKey = t == '{'
				} else {
					b.WriteString(closingOf(rune(t)))
					needComma = true
					skipClose = true
					if depth == 0 {
						done = true
					}
				}
			case '}', ']':
				depth--
				inObj = inObj[:len(inObj)-1]
				b.WriteString(indentTo(depth))
				b.WriteRune(rune(t))
				needComma = true
				expectKey = len(inObj) > 0 && inObj[len(inObj)-1]
				if depth == 0 {
					done = true
				}
			}
		case string:
			sep()
			writeJSONString(&b, t)
			if expectKey && len(inObj) > 0 && inObj[len(inObj)-1] {
				pendingColon = true
				expectKey = false
			} else {
				scalarDone()
			}
		case json.Number:
			sep()
			b.WriteString(t.String())
			scalarDone()
		case bool:
			sep()
			b.WriteString(strconv.FormatBool(t))
			scalarDone()
		case nil:
			sep()
			b.WriteString("null")
			scalarDone()
		}
	}
	if !done {
		return "", errors.New("内容为空")
	}
	return b.String(), nil
}

func indentTo(depth int) string {
	return "\n" + strings.Repeat("  ", depth)
}

func closingOf(open rune) string {
	if open == '{' {
		return "}"
	}
	return "]"
}

// writeJSONString 写出最小转义的 JSON 字符串（含引号）：只转义 JSON
// 规范要求的字符，CJK 与 emoji 等原文保留，非法 UTF-8 换 U+FFFD。
func writeJSONString(b *strings.Builder, s string) {
	b.WriteByte('"')
	for _, r := range s {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				b.WriteString(`\u`)
				hex := strconv.FormatInt(int64(r), 16)
				b.WriteString(strings.Repeat("0", 4-len(hex)))
				b.WriteString(hex)
			} else if r == utf8.RuneError {
				b.WriteString(`\ufffd`)
			} else {
				b.WriteRune(r)
			}
		}
	}
	b.WriteByte('"')
}

// Validate 校验 src 是单个合法 JSON 值；出错时返回错误和指向出错处
// 的 Pos（json 的错误偏移按字节计，这里换算为行/列/rune 偏移）。
// 空文档也报错，由调用方决定是否忽略。
func Validate(src string) (Pos, error) {
	if strings.TrimSpace(src) == "" {
		return Pos{1, 1, 0}, errors.New("内容为空")
	}
	dec := json.NewDecoder(strings.NewReader(src))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		if errors.Is(err, io.EOF) {
			return Pos{1, 1, 0}, errors.New("不完整的 JSON")
		}
		off := errOffset(err)
		if off == 0 {
			off = len(src) // EOF 类错误没有偏移，指到文档末尾
		}
		return posAt(src, off), err
	}
	if dec.More() {
		return posAt(src, int(dec.InputOffset())), errors.New("JSON 后有多余内容")
	}
	return Pos{}, nil
}

// jsonErrText 把标准库的英文错误翻成稳定的中文短语（状态栏展示）。
func jsonErrText(err error) string {
	if err == nil {
		return ""
	}
	msg := err.Error()
	switch {
	case strings.Contains(msg, "unexpected EOF"),
		strings.Contains(msg, "unexpected end of JSON input"):
		return "JSON 不完整"
	case strings.HasPrefix(msg, "invalid character"):
		return "非法字符"
	}
	return msg
}

// errOffset 取出编码/解码错误携带的字节偏移。
func errOffset(err error) int {
	var se *json.SyntaxError
	if errors.As(err, &se) {
		return int(se.Offset)
	}
	var te *json.UnmarshalTypeError
	if errors.As(err, &te) {
		return int(te.Offset)
	}
	return 0
}

// posAt 把字节偏移换成行/列/rune 偏移；偏移越界时钳到文档范围内。
func posAt(src string, byteOff int) Pos {
	if byteOff < 0 {
		byteOff = 0
	}
	if byteOff > len(src) {
		byteOff = len(src)
	}
	prefix := src[:byteOff]
	line, col, runes := 1, 1, 0
	for _, r := range prefix {
		runes++
		if r == '\n' {
			line++
			col = 1
		} else {
			col++
		}
	}
	return Pos{line, col, runes}
}
