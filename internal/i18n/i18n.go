// Package i18n provides minimal zh/en string translation with placeholder
// substitution and English fallback for missing keys.
package i18n

import (
	"strconv"
	"strings"
)

// Lang is a language identifier.
type Lang string

// Supported languages.
const (
	Zh Lang = "zh"
	En Lang = "en"
)

var maps = map[Lang]map[string]string{
	Zh: zh,
	En: en,
}

// T returns the translated string for key in lang. If the key is missing in
// lang, it falls back to English, then to key itself. Positional {0}-style
// placeholders in the message are replaced by args in order.
func T(lang Lang, key string, args ...any) string {
	m, ok := maps[lang]
	if !ok {
		m = en
	}
	s, ok := m[key]
	if !ok {
		s, ok = en[key]
		if !ok {
			s = key
		}
	}
	if len(args) == 0 {
		return s
	}
	return substitute(s, args...)
}

// substitute replaces {0}, {1}, ... placeholders with the corresponding arg.
func substitute(s string, args ...any) string {
	var b strings.Builder
	b.Grow(len(s))
	for i := 0; i < len(s); {
		c := s[i]
		if c != '{' {
			b.WriteByte(c)
			i++
			continue
		}
		end := strings.IndexByte(s[i:], '}')
		if end < 0 {
			b.WriteString(s[i:])
			break
		}
		num, err := strconv.Atoi(s[i+1 : i+end])
		if err != nil || num < 0 || num >= len(args) {
			b.WriteString(s[i : i+end+1])
			i += end + 1
			continue
		}
		b.WriteString(formatArg(args[num]))
		i += end + 1
	}
	return b.String()
}
