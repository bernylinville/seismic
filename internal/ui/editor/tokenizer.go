// Package editor 是自建的 JSON 代码编辑器（ADR 0003）：词法高亮、
// 行号、括号匹配、自动缩进、格式化、校验定位与结构/mapping 补全。
// 搜索 Query DSL 与 REST 控制台共用。
package editor

import "strings"

// TokenType 是 JSON 词法单元的种类。
type TokenType uint8

const (
	// TokPunct 是结构符号 {}[]:,。
	TokPunct TokenType = iota
	// TokKey 是对象键（后随冒号的字符串）。
	TokKey
	// TokString 是字符串值。
	TokString
	// TokNumber 是数字。
	TokNumber
	// TokLiteral 是 true/false/null；宽松模式下也兜住残缺字面量
	// （如输入到一半的 "tru"）和一切非结构字符。
	TokLiteral
)

// Token 是一个词法单元；Start/End 是文档内的 rune 偏移（半开区间）。
// 对截断或非法的 JSON 也保证产出可着色的 token，绝不报错——
// 错误定位是 Validate（format.go）的职责。
type Token struct {
	Type  TokenType
	Start int
	End   int
}

// Tokenize 全量词法分析 src。性能不够时再改增量（ADR 0003）。
func Tokenize(src string) []Token {
	rs := []rune(src)
	var toks []Token
	i := 0
	for i < len(rs) {
		r := rs[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			i++
		case r == '"':
			j, closed := i+1, false
			for j < len(rs) {
				if rs[j] == '\\' {
					j += 2 // 转义连同其字符一起吞掉
					continue
				}
				if rs[j] == '"' {
					closed = true
					break
				}
				j++
			}
			if closed {
				j++ // 收尾引号；被截断则 token 到文档末尾为止
			}
			if j > len(rs) {
				j = len(rs) // 末尾悬空转义时钳回
			}
			toks = append(toks, Token{TokString, i, j})
			i = j
		case isNumberStart(r):
			j := i + 1
			for j < len(rs) && isNumberRune(rs[j]) {
				j++
			}
			toks = append(toks, Token{TokNumber, i, j})
			i = j
		case r == '{' || r == '}' || r == '[' || r == ']' || r == ':' || r == ',':
			toks = append(toks, Token{TokPunct, i, i + 1})
			i++
		default:
			// 宽松词法：true/false/null 尝试精确匹配，其余（残缺
			// 字面量、误粘贴的内容）作为整段字面量 token 着色。
			if lit := matchLiteral(rs, i); lit > 0 {
				toks = append(toks, Token{TokLiteral, i, i + lit})
				i += lit
				continue
			}
			j := i
			for j < len(rs) && !isStructural(rs[j]) {
				j++
			}
			if j == i {
				j++ // 防御：孤立的 + . e 等也算一个字面量
			}
			toks = append(toks, Token{TokLiteral, i, j})
			i = j
		}
	}
	markKeys(toks, rs)
	return toks
}

// markKeys 把后随冒号的字符串 token 标记为键。空白不产 token，
// 所以键与冒号在 token 序列里必然相邻；数组里的 "a", 不能算键。
func markKeys(toks []Token, rs []rune) {
	for i := range toks {
		if toks[i].Type != TokString {
			continue
		}
		next := i + 1
		if next < len(toks) && toks[next].Type == TokPunct &&
			toks[next].Start == toks[i].End && rs[toks[next].Start] == ':' {
			toks[i].Type = TokKey
		}
	}
}

func matchLiteral(rs []rune, i int) int {
	for _, lit := range [...]string{"true", "false", "null"} {
		if len(rs)-i >= len(lit) && string(rs[i:i+len(lit)]) == lit {
			return len(lit)
		}
	}
	return 0
}

func isNumberStart(r rune) bool { return r >= '0' && r <= '9' || r == '-' }

func isNumberRune(r rune) bool {
	return r >= '0' && r <= '9' || r == '.' || r == 'e' || r == 'E' || r == '+' || r == '-'
}

func isStructural(r rune) bool {
	switch r {
	case ' ', '\t', '\n', '\r', '"', '{', '}', '[', ']', ':', ',':
		return true
	}
	return strings.ContainsRune("-+.eE0123456789", r)
}
