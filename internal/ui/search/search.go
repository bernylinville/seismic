// Package search builds the「搜索」module page.
package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/editor"
	"seismic/internal/ui/kit"
)

// defaultQuery is elasticvue 的默认查询体。
const defaultQuery = `{"query":{"query_string":{"query":"*"}},"size":10,"from":0}`

// maxSourceColumns 限制展平 _source 后的动态列数，其余仍可在详情里看到。
const maxSourceColumns = 12

// Module holds the 搜索 module's state.
type Module struct {
	loadedOnce bool

	indexPattern string // 索引 pattern，如 log-* 或逗号分隔多个
	indices      []string
	indicesErr   error

	// 查询编辑器（JSON 高亮/补全）与其文本镜像。
	ed    *editor.Editor
	query string

	mappedKey string // 已注入（或在途）mapping 的 cluster+pattern 键
	// mappingCache 按 cluster+pattern 缓存拍平后的字段表，切回旧 pattern
	// 免再拉；mappingOrder 是插入序，超过 mappingCacheMax 逐出最旧。
	mappingCache map[string]map[string]editor.FieldInfo
	mappingOrder []string
	formatErr    string

	from float64
	size float64

	loading bool
	err     error
	ranOnce bool

	rows         []hitRow
	columns      []string // 展平 _source 的动态列（前 maxSourceColumns 个）
	total        int
	tookMS       int64
	selectedRow  int // 选中行号，-1 为无
	queryParseOK bool
	lang         string // 当前 UI 语言，由 View 每帧刷新
}

// hitRow is one hits.hits entry.
type hitRow struct {
	Index  string
	ID     string
	Score  string
	Source map[string]any
	Raw    json.RawMessage // 原始 _source，详情用
}

// searchResponse shapes the _search reply we read.
type searchResponse struct {
	Took int64 `json:"took"`
	Hits struct {
		Total any `json:"total"` // ES7+ 是 {value}, ES6 是数字
		Hits  []struct {
			Index  string          `json:"_index"`
			ID     string          `json:"_id"`
			Score  *float64        `json:"_score"`
			Source json.RawMessage `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

// New makes the module's state.
func New() *Module {
	m := &Module{query: defaultQuery, from: 0, size: 10, selectedRow: -1}
	m.ed = editor.New()
	m.ed.SetText(defaultQuery)
	m.ed.OnChange(func() { m.query = m.ed.Text() })
	return m
}

// Reset 恢复默认查询和分页。
func (m *Module) Reset() {
	m.ed.SetText(defaultQuery)
	m.query = defaultQuery
	m.from = 0
	m.size = 10
	m.formatErr = ""
}

// searchIndices splits the pattern input into index names/patterns.
func (m *Module) searchIndices() []string {
	var out []string
	for _, p := range strings.Split(m.indexPattern, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}

// matchedCount counts loaded index names the pattern input matches.
func (m *Module) matchedCount() int {
	patterns := m.searchIndices()
	if len(patterns) == 0 {
		return len(m.indices)
	}
	n := 0
	for _, name := range m.indices {
		for _, p := range patterns {
			if ok, _ := matchPattern(p, name); ok {
				n++
				break
			}
		}
	}
	return n
}

// matchPattern is a tiny glob: '*' matches any run.
func matchPattern(pattern, s string) (bool, error) {
	parts := strings.Split(pattern, "*")
	if len(parts) == 1 {
		return pattern == s, nil
	}
	if !strings.HasPrefix(s, parts[0]) || !strings.HasSuffix(s, parts[len(parts)-1]) {
		return false, nil
	}
	rest := s[len(parts[0]) : len(s)-len(parts[len(parts)-1])]
	for _, p := range parts[1 : len(parts)-1] {
		i := strings.Index(rest, p)
		if i < 0 {
			return false, nil
		}
		rest = rest[i+len(p):]
	}
	return true, nil
}

// loadIndices pulls index names once per connection.
func (m *Module) loadIndices(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load("search-indices",
		func() { m.loadedOnce = true },
		func(ctx context.Context) func() {
			var rows []struct {
				Index string `json:"index"`
			}
			_, err := client.CatIndicesInto(ctx, map[string]any{
				"format": "json", "h": "index",
			}, "", &rows)
			var names []string
			if err == nil {
				for _, r := range rows {
					names = append(names, r.Index)
				}
			}
			return func() {
				m.indices, m.indicesErr = names, err
			}
		},
	)
}

// searchBody merges from/size into the edited query text; empty body means
// the text is not valid JSON (or empty) and the caller shows an error.
func (m *Module) searchBody() (json.RawMessage, error) {
	text := strings.TrimSpace(m.query)
	if text == "" {
		return json.RawMessage("{}"), nil
	}
	var body map[string]any
	if err := json.Unmarshal([]byte(text), &body); err != nil {
		return nil, err
	}
	if body == nil {
		body = map[string]any{}
	}
	body["from"] = int(m.from)
	body["size"] = int(m.size)
	out, err := json.Marshal(body)
	if err != nil {
		return nil, err
	}
	return out, nil
}

// run fires the search request.
func (m *Module) run(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	m.loadMapping(h)
	body, err := m.searchBody()
	if err != nil {
		m.formatErr = "查询不是有效的 JSON：" + err.Error()
		return
	}
	m.formatErr = ""
	m.selectedRow = -1
	h.Load("search",
		func() { m.loading = true; m.ranOnce = true },
		func(ctx context.Context) func() {
			var res *searchResponse
			var err error
			if res, err = func() (*searchResponse, error) {
				out := &searchResponse{}
				if _, err := client.SearchInto(ctx, body, m.searchIndices(), nil, out); err != nil {
					return nil, err
				}
				return out, nil
			}(); err != nil {
				res = nil
			}
			return func() {
				m.loading = false
				m.err = err
				if err == nil {
					m.applyResults(res)
				}
			}
		},
	)
}

// applyResults turns the parsed response into rows and dynamic columns.
func (m *Module) applyResults(res *searchResponse) {
	m.total, m.tookMS = totalValue(res.Hits.Total), res.Took
	m.rows, m.columns = m.rows[:0], m.columns[:0]
	seen := map[string]bool{}
	for _, h := range res.Hits.Hits {
		row := hitRow{Index: h.Index, ID: h.ID, Raw: h.Source}
		if h.Score == nil {
			row.Score = "-"
		} else {
			row.Score = strconv.FormatFloat(*h.Score, 'f', -1, 64)
		}
		if len(h.Source) > 0 {
			_ = json.Unmarshal(h.Source, &row.Source)
		}
		for _, k := range flattenKeys(row.Source) {
			if !seen[k] {
				seen[k] = true
				if len(m.columns) < maxSourceColumns {
					m.columns = append(m.columns, k)
				}
			}
		}
		m.rows = append(m.rows, row)
	}
}

// totalValue reads hits.total across ES6 (number) and ES7+ ({value}).
func totalValue(v any) int {
	switch t := v.(type) {
	case float64:
		return int(t)
	case map[string]any:
		if f, ok := t["value"].(float64); ok {
			return int(f)
		}
	}
	return 0
}

// flattenKeys lists _source keys with nested objects flattened to dot paths.
func flattenKeys(src map[string]any) []string {
	var keys []string
	var walk func(prefix string, m map[string]any)
	walk = func(prefix string, m map[string]any) {
		for _, k := range sortedKeys(m) {
			path := k
			if prefix != "" {
				path = prefix + "." + k
			}
			if nested, ok := m[k].(map[string]any); ok && len(nested) > 0 {
				walk(path, nested)
				continue
			}
			keys = append(keys, path)
		}
	}
	walk("", src)
	return keys
}

func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// flatValue renders a flattened key's cell text.
func flatValue(src map[string]any, path string) string {
	var cur any = src
	for _, part := range strings.Split(path, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return ""
		}
		cur, ok = m[part]
		if !ok {
			return ""
		}
	}
	switch v := cur.(type) {
	case nil:
		return "null"
	case string:
		return v
	case bool, float64:
		return fmt.Sprint(v)
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// formatQuery pretty-prints the query text with key order preserved; on
// error it keeps the original text and reports the offending line.
func (m *Module) formatQuery() {
	text := strings.TrimSpace(m.query)
	out, err := editor.FormatJSON(text)
	if err != nil {
		line := 1
		if pos, verr := editor.Validate(text); verr != nil {
			line = pos.Line
		}
		m.formatErr = kit.Tr(m.lang, "格式化失败（第 {0} 行附近）：{1}", strconv.Itoa(line), err.Error())
		return
	}
	m.formatErr = ""
	m.ed.SetText(out)
	m.query = out
}

// mappingCacheMax 限制按 pattern 缓存的字段表个数（每个是拍平的
// FieldInfo map，5k 索引的宽 pattern 也不至于占太多内存）。
const mappingCacheMax = 16

// mappingKey 是 mapping 缓存/去重的键：集群 + pattern。集群维度隔离，
// 切换连接后旧缓存自然失效。
func mappingKey(cluster, pattern string) string { return cluster + "\x00" + pattern }

// loadMapping fetches the pattern's mappings and feeds the editor's
// field-name completion (mapping-aware, see ADR 0003). 命中缓存直接注入；
// 在途/已注入由 mappedKey 去重；换 pattern 会取消在途的旧拉取（同 key
// 槽位，见 App.load），失败时清掉 mappedKey 允许重试。
func (m *Module) loadMapping(h kit.ModuleHost) {
	client := h.ES()
	pattern := strings.TrimSpace(m.indexPattern)
	if client == nil || pattern == "" {
		return
	}
	key := mappingKey(h.ClusterKey(), pattern)
	if key == m.mappedKey {
		return // 已注入或在途
	}
	if fields, ok := m.mappingCache[key]; ok {
		m.mappedKey = key
		m.ed.SetMapping(fields)
		return
	}
	m.mappedKey = key
	h.Load("search-mapping",
		func() {},
		func(ctx context.Context) func() {
			var reply map[string]mappingReply
			_, err := client.RequestJSON(ctx, "GET",
				es.CleanIndexName(pattern)+"/_mapping?allow_no_indices=true&ignore_unavailable=true", nil, &reply)
			return func() {
				if err != nil {
					if m.mappedKey == key {
						m.mappedKey = "" // 拉取失败：允许下次重试
					}
					return
				}
				fields := flattenMappings(reply)
				m.cacheMapping(key, fields)
				m.ed.SetMapping(fields)
			}
		},
	)
}

// cacheMapping 存一份字段表并按插入序逐出最旧的。
func (m *Module) cacheMapping(key string, fields map[string]editor.FieldInfo) {
	if m.mappingCache == nil {
		m.mappingCache = map[string]map[string]editor.FieldInfo{}
	}
	if _, exists := m.mappingCache[key]; !exists {
		m.mappingOrder = append(m.mappingOrder, key)
		if len(m.mappingOrder) > mappingCacheMax {
			delete(m.mappingCache, m.mappingOrder[0])
			m.mappingOrder = m.mappingOrder[1:]
		}
	}
	m.mappingCache[key] = fields
}

// mappingReply is the shape of one index's entry in a _mapping response.
type mappingReply struct {
	Mappings struct {
		Properties map[string]json.RawMessage `json:"properties"`
	} `json:"mappings"`
}

// flattenMappings walks the _mapping reply into dot-path field infos,
// merging the mappings of every index the pattern matched.
func flattenMappings(reply map[string]mappingReply) map[string]editor.FieldInfo {
	fields := map[string]editor.FieldInfo{}
	for _, idx := range reply {
		collectFields(idx.Mappings.Properties, "", fields)
	}
	return fields
}

func collectFields(props map[string]json.RawMessage, prefix string, out map[string]editor.FieldInfo) {
	for name, raw := range props {
		var f struct {
			Type       string                     `json:"type"`
			Properties map[string]json.RawMessage `json:"properties"`
		}
		if err := json.Unmarshal(raw, &f); err != nil {
			continue
		}
		full := name
		if prefix != "" {
			full = prefix + "." + name
		}
		if f.Type != "" {
			out[full] = editor.FieldInfo{Name: full, Type: f.Type}
		}
		if len(f.Properties) > 0 {
			collectFields(f.Properties, full, out)
		}
	}
}

// prettySource renders the selected row's _source for the detail pane.
func (r *hitRow) prettySource() string {
	var out bytes.Buffer
	if len(r.Raw) == 0 || json.Indent(&out, r.Raw, "", "  ") != nil {
		return "{}"
	}
	return out.String()
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	m.lang = h.Lang()
	if !m.loadedOnce && h.ES() != nil {
		m.loadIndices(h)
	}
	t := c.Theme()

	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Text(c, kit.Tr(h.Lang(), "文档搜索")).FontSize(20).Bold()

		// 查询区。
		ui.Column(c).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &m.indexPattern).Placeholder(kit.Tr(h.Lang(), "索引，如 log-* 或 logs-1,logs-2")).Width(240)
				if m.indicesErr != nil {
					ui.Text(c, kit.ErrMessage(m.indicesErr)).FontSize(12).TextColor(ui.Hex("#d64545"))
				} else {
					ui.Text(c, kit.Tr(h.Lang(), "匹配 {0} 个索引", m.matchedCount())).FontSize(12).TextColor(t.TextMuted)
				}
				ui.Spacer(c)
				ui.Text(c, "from").FontSize(12).TextColor(t.TextMuted)
				ui.NumberInput(c, &m.from, 0, 100000, 10).Width(80)
				ui.Text(c, "size").FontSize(12).TextColor(t.TextMuted)
				ui.NumberInput(c, &m.size, 1, 10000, 10).Width(80)
				if ui.PrimaryButton(c, kit.Tr(h.Lang(), "搜索")).Clicked() {
					m.from = 0
					m.run(h)
				}
			})

			// JSON 代码编辑器：高亮/行号/括号匹配/结构+mapping 补全。
			ui.Column(c).Height(190).Border(1, t.Border).Radius(6).Children(func() {
				m.ed.View(c)
			})

			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "格式化 JSON")).Clicked() {
					m.formatQuery()
				}
				if ui.Button(c, kit.Tr(h.Lang(), "重置")).Clicked() {
					m.Reset()
				}
				if m.formatErr != "" {
					ui.Text(c, m.formatErr).FontSize(12).TextColor(ui.Hex("#d64545"))
				}
			})
		})

		// 结果区。
		if !m.ranOnce {
			ui.Text(c, kit.Tr(h.Lang(), "填好索引和查询，点「搜索」")).TextColor(t.TextMuted)
			return
		}
		if m.loading {
			ui.Text(c, kit.Tr(h.Lang(), "搜索中…")).TextColor(t.TextMuted)
			return
		}
		if m.err != nil {
			ui.Text(c, kit.ErrMessage(m.err)).FontSize(12).TextColor(ui.Hex("#d64545"))
			return
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "共 {0} 条 · {1} ms", m.total, m.tookMS)).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, kit.Tr(h.Lang(), "上一页")).Disabled(m.from <= 0).Clicked() {
				m.from -= m.size
				if m.from < 0 {
					m.from = 0
				}
				m.run(h)
			}
			if ui.Button(c, kit.Tr(h.Lang(), "下一页")).Disabled(int(m.from)+int(m.size) >= m.total).Clicked() {
				m.from += m.size
				m.run(h)
			}
		})

		if len(m.rows) == 0 {
			ui.Text(c, kit.Tr(h.Lang(), "没搜到任何文档")).TextColor(t.TextMuted)
			return
		}
		if m.selectedRow >= len(m.rows) {
			m.selectedRow = -1
		}
		cols := make([]ui.TableColumn, 0, 3+len(m.columns))
		cols = append(cols,
			ui.TableColumn{Title: "_index", Width: 140},
			ui.TableColumn{Title: "_id", Width: 140},
			ui.TableColumn{Title: "_score", Width: 70, Align: ui.End},
		)
		for _, k := range m.columns {
			cols = append(cols, ui.TableColumn{Title: k})
		}
		listState := ui.ListState{Selected: &m.selectedRow}
		ui.Table(c, &listState, cols, len(m.rows), func(row, col int) {
			r := m.rows[row]
			switch {
			case col == 0:
				ui.Text(c, r.Index).FontSize(12).SingleLine()
			case col == 1:
				ui.Text(c, r.ID).FontSize(12).SingleLine()
			case col == 2:
				ui.Text(c, r.Score).FontSize(12)
			default:
				ui.Text(c, flatValue(r.Source, m.columns[col-3])).FontSize(12).SingleLine()
			}
		}).Grow(1)

		if m.selectedRow >= 0 {
			r := m.rows[m.selectedRow]
			ui.Column(c).Height(180).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
				Background(t.Surface).Padding(12, 16, 12, 16).Gap(6).Children(func() {
				ui.Text(c, r.Index+" / "+r.ID).Bold().FontSize(12)
				ui.Text(c, r.prettySource()).Font("monospace").FontSize(12)
			})
		}
	})
}
