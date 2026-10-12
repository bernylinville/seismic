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

	// 查询编辑区：将来整体替换为 editor.Editor 代码编辑器组件（另行交付）。
	query     string
	formatErr string

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
	return &Module{query: defaultQuery, from: 0, size: 10, selectedRow: -1}
}

// Reset 恢复默认查询和分页。
func (m *Module) Reset() {
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
	h.Load(
		func() { m.loadedOnce = true },
		func() func() {
			resp, err := client.CatIndices(context.Background(), map[string]any{
				"format": "json", "h": "index",
			}, "")
			var names []string
			if err == nil {
				var rows []struct {
					Index string `json:"index"`
				}
				if err = resp.JSON(&rows); err == nil {
					for _, r := range rows {
						names = append(names, r.Index)
					}
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
	body, err := m.searchBody()
	if err != nil {
		m.formatErr = "查询不是有效的 JSON：" + err.Error()
		return
	}
	m.formatErr = ""
	m.selectedRow = -1
	h.Load(
		func() { m.loading = true; m.ranOnce = true },
		func() func() {
			resp, err := client.Search(context.Background(), body, m.searchIndices(), nil)
			var res *searchResponse
			if err == nil {
				res = &searchResponse{}
				if err = resp.JSON(res); err != nil {
					res = nil
				}
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

// formatQuery pretty-prints the query text; on error it keeps the original
// text and reports the offending line.
func (m *Module) formatQuery() {
	text := strings.TrimSpace(m.query)
	var out bytes.Buffer
	if err := json.Indent(&out, []byte(text), "", "  "); err != nil {
		m.formatErr = "格式化失败（第 " + strconv.Itoa(errorLine(text, err)) + " 行附近）：" + err.Error()
		return
	}
	m.formatErr = ""
	m.query = out.String()
}

// errorLine maps a json error offset to a 1-based line number.
func errorLine(text string, err error) int {
	se, ok := err.(*json.SyntaxError)
	if !ok || se.Offset <= 0 || se.Offset > int64(len(text)) {
		return 1
	}
	return 1 + strings.Count(text[:se.Offset], "\n")
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
	if !m.loadedOnce && h.ES() != nil {
		m.loadIndices(h)
	}
	t := c.Theme()

	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Text(c, "文档搜索").FontSize(20).Bold()

		// 查询区。
		ui.Column(c).Gap(8).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.TextInput(c, &m.indexPattern).Placeholder("索引，如 log-* 或 logs-1,logs-2").Width(240)
				if m.indicesErr != nil {
					ui.Text(c, kit.ErrMessage(m.indicesErr)).FontSize(12).TextColor(ui.Hex("#d64545"))
				} else {
					ui.Text(c, fmt.Sprintf("匹配 %d 个索引", m.matchedCount())).FontSize(12).TextColor(t.TextMuted)
				}
				ui.Spacer(c)
				ui.Text(c, "from").FontSize(12).TextColor(t.TextMuted)
				ui.NumberInput(c, &m.from, 0, 100000, 10).Width(80)
				ui.Text(c, "size").FontSize(12).TextColor(t.TextMuted)
				ui.NumberInput(c, &m.size, 1, 10000, 10).Width(80)
				if ui.PrimaryButton(c, "搜索").Clicked() {
					m.from = 0
					m.run(h)
				}
			})

			// 查询编辑：目前用等宽 TextArea；将来替换为 editor.Editor
			// 代码编辑器组件（高亮/行号，另行交付），状态字段 query 保持不变。
			ui.TextArea(c, &m.query).Height(170).Font("monospace")

			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "格式化 JSON").Clicked() {
					m.formatQuery()
				}
				if ui.Button(c, "重置").Clicked() {
					m.Reset()
				}
				if m.formatErr != "" {
					ui.Text(c, m.formatErr).FontSize(12).TextColor(ui.Hex("#d64545"))
				}
			})
		})

		// 结果区。
		if !m.ranOnce {
			ui.Text(c, "输入索引 pattern 与查询后点「搜索」").TextColor(t.TextMuted)
			return
		}
		if m.loading {
			ui.Text(c, "搜索中…").TextColor(t.TextMuted)
			return
		}
		if m.err != nil {
			ui.Text(c, kit.ErrMessage(m.err)).FontSize(12).TextColor(ui.Hex("#d64545"))
			return
		}
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmt.Sprintf("共 %d 条 · %d ms", m.total, m.tookMS)).FontSize(12).TextColor(t.TextMuted)
			ui.Spacer(c)
			if ui.Button(c, "上一页").Disabled(m.from <= 0).Clicked() {
				m.from -= m.size
				if m.from < 0 {
					m.from = 0
				}
				m.run(h)
			}
			if ui.Button(c, "下一页").Disabled(int(m.from)+int(m.size) >= m.total).Clicked() {
				m.from += m.size
				m.run(h)
			}
		})

		if len(m.rows) == 0 {
			ui.Text(c, "没有命中文档").TextColor(t.TextMuted)
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
