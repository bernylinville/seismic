package kit

import (
	"fmt"
	"strconv"
	"strings"
)

// en holds the registered English translations: Chinese source → English.
var en = map[string]string{}

// RegisterLang registers English translations for a package's UI strings.
// Keys are the Chinese source strings used at call sites; values are the
// English renderings.
func RegisterLang(m map[string]string) {
	for k, v := range m {
		en[k] = v
	}
}

// Tr renders key in lang. In Chinese (or any non-English language) the key
// itself is the text; in English it looks up the registered translation and
// falls back to the key when none exists. Positional {0} placeholders are
// replaced by args in order.
func Tr(lang, key string, args ...any) string {
	s := key
	if lang == "en" {
		if v, ok := en[key]; ok {
			s = v
		}
	}
	for i, a := range args {
		s = strings.ReplaceAll(s, "{"+strconv.Itoa(i)+"}", toString(a))
	}
	return s
}

func toString(a any) string {
	if s, ok := a.(string); ok {
		return s
	}
	return fmt.Sprint(a)
}
