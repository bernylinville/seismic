package editor

import (
	"sort"
	"strings"
)

// PosKind 是光标处可补全的位置类型。
type PosKind uint8

const (
	// PosNone 无补全（冒号后、值后等待逗号等）。
	PosNone PosKind = iota
	// PosKey 键位置：对象里正在写键名。
	PosKey
	// PosValue 值位置：冒号后、数组里或文档顶层。
	PosValue
)

// FieldInfo 是 mapping 注入的一个字段（SetMapping 的元素）。
type FieldInfo struct {
	Name string
	Type string // text、keyword、long、date、boolean…
}

// CompletionContext 是光标处的结构推断结果。
// 所有偏移均为 rune 偏移。
type CompletionContext struct {
	Kind PosKind
	// Path 是包含光标的对象键路径（数组不贡献路径段），
	// 如 {"query":{"bool":{█ → ["query","bool"]。
	Path []string
	// Prefix 是光标前的半成品输入（字符串内容或裸词）。
	Prefix string
	// InString：光标在一个未闭合的字符串里（正在写键或字符串值）。
	InString bool
	// QuotePos / WordPos 是该半成品的起始引号 / 裸词起点。
	QuotePos int
	WordPos  int
	HasWord  bool
	// Siblings 是光标所在对象里已写完的键（避免重复建议）。
	Siblings []string
	// DocKeys 是整篇文档（含光标后）出现过的键，按首次出现排序。
	DocKeys []string
	// FieldName：光标所在对象由叶子查询（match/term/…）打开，
	// 键位置应建议 mapping 字段名而非 DSL 关键字。
	FieldName bool
}

// 容器扫描状态。
const (
	cfKey   = iota + 1 // 对象里等键名
	cfColon            // 键名后等冒号
	cfValue            // 等值（冒号后/数组内/顶层）
	cfComma            // 值后等逗号或收尾
)

type cframe struct {
	obj   bool
	key   string // 打开该容器时的键名（数组/顶层为 ""）
	state int
	keys  []string // 对象里已出现的键
	cur   string   // 对象里最近解析出的键（给子容器命名）
}

// ContextAt 推断 src 中 caret（rune 偏移）处的补全上下文。
// 对残缺 JSON 宽松处理：只看结构符号与字符串。
func ContextAt(src string, caret int) CompletionContext {
	rs := []rune(src)
	if caret < 0 {
		caret = 0
	}
	if caret > len(rs) {
		caret = len(rs)
	}
	ctx := CompletionContext{}
	stack := []cframe{{state: cfValue}} // 顶层：整个文档是一个值
	var wordKind PosKind

scan:
	for i := 0; i < caret; i++ { // 循环体内推进 i，保留经典写法
		top := &stack[len(stack)-1]
		r := rs[i]
		switch {
		case r == ' ' || r == '\t' || r == '\n' || r == '\r':
			continue
		case r == '{':
			ctx.HasWord, ctx.Prefix = false, ""
			stack = append(stack, cframe{obj: true, state: cfKey, key: top.cur})
		case r == '[':
			ctx.HasWord, ctx.Prefix = false, ""
			stack = append(stack, cframe{state: cfValue, key: top.cur})
		case r == '}':
			ctx.HasWord, ctx.Prefix = false, ""
			if len(stack) > 1 && top.obj {
				stack = stack[:len(stack)-1]
			}
		case r == ']':
			ctx.HasWord, ctx.Prefix = false, ""
			if len(stack) > 1 && !top.obj {
				stack = stack[:len(stack)-1]
			}
		case r == '"':
			j := i + 1
			for j < caret && rs[j] != '"' {
				if rs[j] == '\\' {
					j += 2
				} else {
					j++
				}
			}
			if j >= caret {
				// 未闭合：光标就在这个字符串里。
				ctx.InString = true
				ctx.QuotePos = i
				ctx.Prefix = unquoteLoose(string(rs[i+1 : caret]))
				break scan
			}
			s := unquoteLoose(string(rs[i+1 : j]))
			ctx.HasWord, ctx.Prefix = false, ""
			if top.obj && top.state == cfKey {
				top.keys = append(top.keys, s)
				top.cur = s
				top.state = cfColon
			} else {
				top.state = cfComma
			}
			i = j
		case r == ':':
			ctx.HasWord, ctx.Prefix = false, ""
			if top.state == cfColon {
				top.state = cfValue
			}
		case r == ',':
			ctx.HasWord, ctx.Prefix = false, ""
			if top.state == cfComma {
				if top.obj {
					top.state = cfKey
				} else {
					top.state = cfValue
				}
			}
		case isWordRune(r):
			j := i
			for j < caret && isWordRune(rs[j]) {
				j++
			}
			if top.state == cfKey || top.state == cfValue {
				wordKind = PosKey
				if top.state == cfValue {
					wordKind = PosValue
				}
				ctx.HasWord = true
				ctx.WordPos = i
				ctx.Prefix = string(rs[i:j])
			}
			if top.state == cfValue {
				top.state = cfComma
			}
			i = j - 1
		}
	}
	// 未闭合字符串与裸词之外的场合，由栈顶状态定位置；
	// InString 时栈顶状态正是字符串开启前的状态，同样适用。
	top := &stack[len(stack)-1]
	switch top.state {
	case cfKey:
		wordKind = PosKey
	case cfValue:
		wordKind = PosValue
	default:
		if !ctx.HasWord {
			wordKind = PosNone
		}
	}
	if top.obj {
		ctx.Siblings = top.keys
		ctx.FieldName = leafQueries[top.key]
	}
	for _, fr := range stack[1:] {
		if fr.obj && fr.key != "" {
			ctx.Path = append(ctx.Path, fr.key)
		}
	}
	ctx.Kind = wordKind
	ctx.DocKeys = documentKeys(src)
	return ctx
}

// unquoteLoose 尽力解一层字符串转义（只处理 \" 与 \\，足够补全用）。
func unquoteLoose(s string) string {
	if !strings.ContainsAny(s, `\\`) {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] == '\\' && i+1 < len(s) {
			i++
			if s[i] != '"' && s[i] != '\\' {
				b.WriteByte('\\')
			}
			b.WriteByte(s[i])
			continue
		}
		b.WriteByte(s[i])
	}
	return b.String()
}

func isWordRune(r rune) bool {
	return r == '_' || r == '-' || r == '.' ||
		r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
}

// documentKeys 收集整篇文档里所有键（含光标之后），保序去重。
func documentKeys(src string) []string {
	var out []string
	seen := map[string]bool{}
	rs := []rune(src)
	for _, tok := range Tokenize(src) {
		if tok.Type != TokKey {
			continue
		}
		k := unquoteLoose(string(rs[tok.Start+1 : tok.End-1]))
		if !seen[k] {
			seen[k] = true
			out = append(out, k)
		}
	}
	return out
}

// SuggestionKind 决定插入时的引号处理。
type SuggestionKind uint8

const (
	// SuggKey 键建议：Insert 形如 `name": `（不含起始引号，
	// 不在字符串内插入时自动补起始引号）。
	SuggKey SuggestionKind = iota
	// SuggValue 值建议：Insert 原样插入。
	SuggValue
)

// Suggestion 是一条补全建议。
type Suggestion struct {
	Label     string // 列表展示
	Insert    string // 替换半成品后插入的文本
	Detail    string // 右侧徽标/说明
	Kind      SuggestionKind
	CaretBack int // 插入后光标回退的 rune 数（如 "" 中间）
}

// leafQueries 打开的对象是“字段名位置”。
var leafQueries = map[string]bool{
	"match": true, "match_phrase": true, "match_bool_prefix": true,
	"multi_match": true, "term": true, "terms": true, "range": true,
	"prefix": true, "wildcard": true, "regexp": true, "fuzzy": true,
	"exists": true, "terms_set": true, "span_term": true,
}

// dslDictionary 是内置 Query DSL 词典（有序）。
var dslDictionary = [...]struct{ label, detail string }{
	{"query", "查询体"},
	{"size", "返回条数"},
	{"from", "起始偏移"},
	{"sort", "排序"},
	{"_source", "字段过滤"},
	{"track_total_hits", "精确总数"},
	{"min_score", "最小分数"},
	{"timeout", "超时"},
	{"highlight", "高亮"},
	{"fields", "提取字段"},
	{"aggs", "聚合定义"},
	{"post_filter", "后过滤"},
	{"search_after", "分页游标"},
	{"collapse", "结果折叠"},
	{"knn", "KNN 向量检索"},
	{"explain", "解释评分"},
	{"suggest", "建议"},
	{"boost", "权重"},
	{"match", "全文匹配"},
	{"match_phrase", "短语匹配"},
	{"match_bool_prefix", "前缀布尔匹配"},
	{"multi_match", "多字段匹配"},
	{"query_string", "查询字符串"},
	{"simple_query_string", "简单查询字符串"},
	{"term", "精确匹配"},
	{"terms", "多值精确匹配"},
	{"range", "范围过滤"},
	{"prefix", "前缀匹配"},
	{"wildcard", "通配符匹配"},
	{"regexp", "正则匹配"},
	{"fuzzy", "模糊匹配"},
	{"exists", "字段存在"},
	{"ids", "按 _id 匹配"},
	{"bool", "组合查询"},
	{"must", "必须匹配"},
	{"should", "应该匹配"},
	{"must_not", "必须不匹配"},
	{"filter", "过滤（不评分）"},
	{"minimum_should_match", "最小 should 命中数"},
	{"constant_score", "常量评分"},
	{"dis_max", "分离最大化"},
	{"function_score", "函数打分"},
	{"script_score", "脚本打分"},
	{"nested", "嵌套查询"},
	{"has_child", "子文档查询"},
	{"has_parent", "父文档查询"},
}

// valueSuggestions 是值位置的固定建议。
var valueSuggestions = [...]Suggestion{
	{Label: "true", Insert: "true", Detail: "布尔值", Kind: SuggValue},
	{Label: "false", Insert: "false", Detail: "布尔值", Kind: SuggValue},
	{Label: "null", Insert: "null", Detail: "空值", Kind: SuggValue},
	{Label: `""`, Insert: `""`, Detail: "字符串", Kind: SuggValue, CaretBack: 1},
}

// Suggest 按 ctx 生成建议列表；mapping 非空且处于字段名位置时
// 优先给真实字段（带类型徽标）。返回空切片表示无可补全。
func Suggest(ctx *CompletionContext, mapping map[string]FieldInfo) []Suggestion {
	if ctx.Kind == PosNone {
		return nil
	}
	prefix := strings.ToLower(ctx.Prefix)
	var out []Suggestion
	switch {
	case ctx.Kind == PosValue:
		for _, s := range valueSuggestions {
			if s.Label == `""` && prefix != "" {
				continue // 已在输入内容时空串建议无意义
			}
			if strings.HasPrefix(s.Label, prefix) {
				out = append(out, s)
			}
		}
	case ctx.FieldName && len(mapping) > 0:
		names := make([]string, 0, len(mapping))
		for name := range mapping {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if !matchPrefix(name, prefix) || isSibling(ctx, name) {
				continue
			}
			out = append(out, Suggestion{
				Label:  name,
				Insert: name + `": `,
				Detail: mapping[name].Type,
				Kind:   SuggKey,
			})
		}
	default:
		for _, d := range dslDictionary {
			if !matchPrefix(d.label, prefix) || isSibling(ctx, d.label) {
				continue
			}
			out = append(out, Suggestion{
				Label: d.label, Insert: d.label + `": `, Detail: d.detail, Kind: SuggKey,
			})
		}
		dict := map[string]bool{}
		for _, d := range dslDictionary {
			dict[d.label] = true
		}
		for _, k := range ctx.DocKeys {
			if dict[k] || isSibling(ctx, k) || !matchPrefix(k, prefix) {
				continue
			}
			out = append(out, Suggestion{
				Label: k, Insert: k + `": `, Detail: "文档中已出现", Kind: SuggKey,
			})
		}
	}
	return out
}

func matchPrefix(s, prefix string) bool {
	return strings.HasPrefix(strings.ToLower(s), prefix)
}

func isSibling(ctx *CompletionContext, key string) bool {
	for _, k := range ctx.Siblings {
		if k == key {
			return true
		}
	}
	return false
}
