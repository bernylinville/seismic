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
func FormatJSON(src string) (string, error) {
	dec := json.NewDecoder(strings.NewReader(src))
	dec.UseNumber()
	var b strings.Builder
	depth := 0
	nl := true      // 下一个 token 前需要换行+缩进
	skipClose := false // 内联收尾的容器，其闭合符作为 token 到达时跳过
	done := false   // 顶层值已完成（之后再有 token 即多余内容）
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
			if d, ok := tok.(json.Delim); ok && (d == '}' || d == ']') {
				skipClose = false
				continue
			}
			skipClose = false
		}
		switch t := tok.(type) {
		case json.Delim:
			switch t {
			case '{', '[':
				if nl {
					b.WriteString(indentTo(depth))
				}
				b.WriteRune(rune(t))
				if dec.More() {
					depth++
					nl = true
				} else {
					b.WriteString(closingOf(rune(t)))
					nl = true
					skipClose = true
				}
			case '}', ']':
				depth--
				if nl {
					b.WriteString(indentTo(depth))
				}
				b.WriteRune(rune(t))
				nl = true
			case ':':
				b.WriteString(": ")
				nl = false
			case ',':
				b.WriteByte(',')
				nl = true
			}
		case string:
			if nl {
				b.WriteString(indentTo(depth))
			}
			writeJSONString(&b, t)
			nl = false
		case json.Number:
			if nl {
				b.WriteString(indentTo(depth))
			}
			b.WriteString(t.String())
			nl = false
		case bool:
			if nl {
				b.WriteString(indentTo(depth))
			}
			b.WriteString(strconv.FormatBool(t))
			nl = false
		case nil:
			if nl {
				b.WriteString(indentTo(depth))
			}
			b.WriteString("null")
			nl = false
		}
		if depth == 0 {
			done = true
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
		return posAt(src, errOffset(err)), err
	}
	if dec.More() {
		return posAt(src, int(dec.InputOffset())), errors.New("JSON 后有多余内容")
	}
	return Pos{}, nil
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
