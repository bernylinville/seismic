// Package rest builds the「控制台」module page.
package rest

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"seismic/internal/db"
	"seismic/internal/es"
	"seismic/internal/ui/editor"
	"seismic/internal/ui/kit"
)

// httpMethods mirrors elasticvue's HTTP_METHODS minus PATCH (not sent by
// any Seismic flow).
var httpMethods = []string{"GET", "POST", "PUT", "DELETE", "HEAD", "OPTIONS"}

const defaultPath = "_cat/indices"

func newTab() tab { return tab{method: "GET", path: defaultPath} }

// tab is one request tab: its own method, path, body and last response.
type tab struct {
	method string
	path   string
	body   string
	ed     *editor.Editor // body 编辑器，nil 时惰性创建

	sending bool

	sent    bool
	status  int
	durMS   int64
	respErr string
	resp    string
}

// Module holds the 控制台 module's state.
type Module struct {
	tabs   []tab
	active int

	// saveName is the name field for saving the current tab as a query.
	saveName string

	history     []db.HistoryEntry
	saved       []db.SavedQuery
	listsErr    string
	cluster     string // the cluster the lists below belong to
	histLoading bool
}

// New makes the module's state.
func New() *Module {
	return &Module{tabs: []tab{newTab()}}
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()

	// Per-cluster data: reload the lists and reset tabs whenever the cluster
	// changes, so history and responses never leak across clusters.
	if key := h.ClusterKey(); key != m.cluster {
		m.cluster = key
		m.history, m.saved, m.listsErr = nil, nil, ""
		m.tabs, m.active = []tab{newTab()}, 0
		m.reloadLists(h)
	}

	ui.Column(c).Fill().Padding(16).Gap(12).Children(func() {
		ui.Row(c).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "控制台")).FontSize(20).Bold()
		})

		ui.Row(c).Gap(8).Children(func() {
			m.tabStrip(c)
			if ui.Button(c, kit.Tr(h.Lang(), "新建标签")).Clicked() {
				m.tabs = append(m.tabs, newTab())
				m.active = len(m.tabs) - 1
			}
			if ui.Button(c, kit.Tr(h.Lang(), "关闭标签")).Clicked() {
				m.closeTab()
			}
		})

		ui.Row(c).Fill().Gap(12).Children(func() {
			// 左侧：请求编辑 + 响应。
			ui.Column(c).Grow(1).Gap(8).Children(func() {
				m.requestView(h, c)
				m.responseView(c, t, h.Lang())
			})
			// 右侧：历史 + 保存的查询。
			m.sideView(h, c, t)
		})
	})
}

// tabStrip renders the ui.Tabs row for the open tabs.
func (m *Module) tabStrip(c *ui.Context) {
	labels := make([]string, len(m.tabs))
	for i := range m.tabs {
		labels[i] = tabLabel(&m.tabs[i])
	}
	ui.Tabs(c, &m.active, labels...)
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
}

func tabLabel(tb *tab) string {
	p := tb.path
	if r := []rune(p); len(r) > 18 {
		p = string(r[:17]) + "…"
	}
	return tb.method + " " + p
}

func (m *Module) closeTab() {
	if len(m.tabs) == 1 {
		m.tabs[0] = newTab()
		return
	}
	m.tabs = append(m.tabs[:m.active], m.tabs[m.active+1:]...)
	if m.active >= len(m.tabs) {
		m.active = len(m.tabs) - 1
	}
}

// requestView renders method/path/body of the active tab plus the send row.
func (m *Module) requestView(h kit.ModuleHost, c *ui.Context) {
	tb := &m.tabs[m.active]
	t := c.Theme()

	ui.Column(c).Border(1, t.Border).Radius(8).Padding(12).Gap(8).Children(func() {
		ui.Row(c).Gap(8).Children(func() {
			ui.Select(c, &tb.method, httpMethods).Width(120)
			if path := ui.TextInput(c, &tb.path).Label(kit.Tr(h.Lang(), "路径")).Grow(1); path.Submitted() {
				m.send(h)
			}
			if ui.Button(c, kit.Tr(h.Lang(), "发送")).Clicked() {
				m.send(h)
			}
		})

		if tb.method == "GET" || tb.method == "HEAD" {
			ui.Text(c, kit.Tr(h.Lang(), "{0} 请求不带正文；参数写在路径里，如 _cat/indices?v=true", tb.method)).
				TextColor(t.TextMuted).FontSize(12)
		} else {
			ui.Column(c).Height(150).Border(1, t.Border).Radius(6).Children(func() {
				m.editorFor(tb).View(c)
			})
		}

		ui.Row(c).Gap(8).Children(func() {
			ui.TextInput(c, &m.saveName).Label(kit.Tr(h.Lang(), "查询名称")).Placeholder(kit.Tr(h.Lang(), "查询名称")).Width(200)
			if ui.Button(c, kit.Tr(h.Lang(), "保存查询")).Clicked() {
				m.saveQuery(h)
			}
			if tb.sending {
				ui.Text(c, kit.Tr(h.Lang(), "发送中…")).TextColor(t.TextMuted)
			}
		})
	})
}

// responseView renders the active tab's last response.
func (m *Module) responseView(c *ui.Context, t *ui.Theme, lang string) {
	tb := &m.tabs[m.active]

	ui.Column(c).Grow(1).Border(1, t.Border).Radius(8).Padding(12).Gap(8).Children(func() {
		if !tb.sent {
			ui.Text(c, kit.Tr(lang, "还没发过请求")).TextColor(t.TextMuted)
			return
		}
		ui.Row(c).Gap(12).Children(func() {
			ui.Text(c, fmt.Sprintf("%d", tb.status)).Bold().TextColor(statusColor(tb.status))
			ui.Text(c, kit.Tr(lang, "耗时 {0} ms", tb.durMS)).TextColor(t.TextMuted).FontSize(12)
			if tb.respErr != "" {
				ui.Text(c, tb.respErr).TextColor(ui.Hex("#d64545")).FontSize(12)
			}
		})
		out := tb.resp
		if out == "" {
			out = tb.respErr
		}
		ui.TextArea(c, &out).Font("monospace").ReadOnly(true).Lines(4, 20).Grow(1)
	})
}

// statusColor maps the response code to the health palette.
func statusColor(code int) ui.Color {
	switch {
	case code >= 200 && code < 300:
		return ui.Hex("#2f9e63")
	case code >= 400 && code < 500:
		return ui.Hex("#d9a406")
	case code >= 500:
		return ui.Hex("#d64545")
	default:
		return ui.Hex("#d9a406")
	}
}

// sideView renders history and saved queries.
func (m *Module) sideView(h kit.ModuleHost, c *ui.Context, t *ui.Theme) {
	ui.Column(c).Width(280).Gap(12).Children(func() {
		// 历史
		ui.Column(c).Border(1, t.Border).Radius(8).Padding(12).Gap(8).Grow(1).Children(func() {
			ui.Row(c).Children(func() {
				ui.Text(c, kit.Tr(h.Lang(), "历史")).Bold().Grow(1)
				if ui.Button(c, kit.Tr(h.Lang(), "清空历史")).Clicked() {
					m.clearHistory(h)
				}
			})
			if m.listsErr != "" {
				ui.Text(c, m.listsErr).TextColor(ui.Hex("#d64545")).FontSize(12)
			}
			if len(m.history) == 0 {
				if m.histLoading {
					ui.Text(c, kit.Tr(h.Lang(), "加载中…")).TextColor(t.TextMuted)
				} else {
					ui.Text(c, kit.Tr(h.Lang(), "发过的请求会出现在这里")).TextColor(t.TextMuted)
				}
				return
			}
			ui.List(c, nil, len(m.history), func(i int) {
				e := m.history[i]
				row := ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, e.Method).TextColor(statusColor(e.Status)).FontSize(12).Width(52)
					ui.Text(c, e.Path).FontSize(12).Grow(1)
					ui.Text(c, fmt.Sprintf("%d", e.Status)).TextColor(statusColor(e.Status)).FontSize(12)
				})
				if row.Clicked() {
					m.applyToActive(e.Method, e.Path, e.Body)
				}
			}).Grow(1)
		})

		// 保存的查询
		ui.Column(c).Border(1, t.Border).Radius(8).Padding(12).Gap(8).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "保存的查询")).Bold()
			if len(m.saved) == 0 {
				ui.Text(c, kit.Tr(h.Lang(), "保存常用请求，一键再发")).TextColor(t.TextMuted)
				return
			}
			ui.List(c, nil, len(m.saved), func(i int) {
				q := m.saved[i]
				row := ui.Row(c).Gap(8).Children(func() {
					ui.Text(c, q.Name).FontSize(12).Grow(1)
					ui.Text(c, q.Method).TextColor(t.TextMuted).FontSize(12)
					if ui.Button(c, kit.Tr(h.Lang(), "删除")).Clicked() {
						m.deleteQuery(h, q.ID)
					}
				})
				if row.Clicked() {
					m.applyToActive(q.Method, q.Path, q.Body)
				}
			})
		})
	})
}

func (m *Module) applyToActive(method, path, body string) {
	tb := &m.tabs[m.active]
	tb.method, tb.path, tb.body = method, path, body
	if tb.ed != nil {
		tb.ed.SetText(body)
	}
}

// editorFor returns the tab's JSON editor, creating it on first use and
// syncing edits back into the tab's body.
func (m *Module) editorFor(tb *tab) *editor.Editor {
	if tb.ed == nil {
		tb.ed = editor.New()
		tb.ed.SetText(tb.body)
		t := tb
		tb.ed.OnChange(func() { t.body = t.ed.Text() })
	}
	return tb.ed
}

// send runs the active tab's request off-thread, records history and
// reloads the history list.
func (m *Module) send(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	tb := &m.tabs[m.active]
	method, path, body := tb.method, tb.path, tb.body
	idx := m.active
	body = strings.TrimSpace(body)
	var param any
	if body != "" && method != "GET" && method != "HEAD" {
		param = body
	}
	key := h.ClusterKey()

	tb.sending = true
	h.Load(
		func() {},
		func() func() {
			start := time.Now()
			resp, err := client.Request(context.Background(), method, path, param)

			dur := time.Since(start).Milliseconds()

			status := 0
			var out []byte
			var errMsg string
			switch {
			case err == nil:
				status = resp.StatusCode
				out = resp.Body
			default:
				if httpErr, ok := err.(*es.HTTPError); ok {
					status = httpErr.Code
					out = httpErr.Body
				}
				errMsg = kit.ErrMessage(err)
			}
			formatted := formatJSON(out)

			if h.DB() != nil {
				_, _ = h.DB().AddHistory(context.Background(), db.HistoryEntry{
					ClusterUUID: key,
					Method:      method,
					Path:        path,
					Body:        body,
					Status:      status,
					DurationMS:  dur,
					CreatedAt:   time.Now().UnixMilli(),
				})
			}
			hist := m.listHistory(h)

			return func() {
				m.history = hist
				if idx >= len(m.tabs) {
					return
				}
				tb := &m.tabs[idx]
				tb.sending = false
				tb.sent = true
				tb.status = status
				tb.durMS = dur
				tb.respErr = errMsg
				tb.resp = formatted
			}
		},
	)
}

// formatJSON indents valid JSON payloads, and leaves anything else as is.
func formatJSON(b []byte) string {
	if len(bytes.TrimSpace(b)) == 0 {
		return ""
	}
	var buf bytes.Buffer
	if err := json.Indent(&buf, b, "", "  "); err != nil {
		return string(b)
	}
	return buf.String()
}

// saveQuery stores the active tab under m.saveName.
func (m *Module) saveQuery(h kit.ModuleHost) {
	store := h.DB()
	name := strings.TrimSpace(m.saveName)
	if store == nil || name == "" {
		return
	}
	tb := &m.tabs[m.active]
	q := db.SavedQuery{
		ClusterUUID: h.ClusterKey(),
		Name:        name,
		Method:      tb.method,
		Path:        tb.path,
		Body:        tb.body,
	}
	h.Load(
		func() {},
		func() func() {
			_, err := store.SaveQuery(context.Background(), q)
			saved := m.listSaved(h)
			return func() {
				if err != nil {
					m.listsErr = kit.ErrMessage(err)
					return
				}
				m.listsErr = ""
				m.saved = saved
			}
		},
	)
}

func (m *Module) deleteQuery(h kit.ModuleHost, id int64) {
	store := h.DB()
	if store == nil {
		return
	}
	h.Load(
		func() {},
		func() func() {
			err := store.DeleteQuery(context.Background(), h.ClusterKey(), id)
			saved := m.listSaved(h)
			return func() {
				if err != nil {
					m.listsErr = kit.ErrMessage(err)
					return
				}
				m.listsErr = ""
				m.saved = saved
			}
		},
	)
}

func (m *Module) clearHistory(h kit.ModuleHost) {
	store := h.DB()
	if store == nil {
		return
	}
	h.Load(
		func() {},
		func() func() {
			err := store.ClearHistory(context.Background(), h.ClusterKey())
			return func() {
				if err != nil {
					m.listsErr = kit.ErrMessage(err)
					return
				}
				m.listsErr = ""
				m.history = nil
			}
		},
	)
}

// reloadLists refreshes history and saved queries for the active cluster.
func (m *Module) reloadLists(h kit.ModuleHost) {
	m.histLoading = true
	h.Load(
		func() {},
		func() func() {
			hist := m.listHistory(h)
			saved := m.listSaved(h)
			return func() {
				m.histLoading = false
				m.history = hist
				m.saved = saved
			}
		},
	)
}

func (m *Module) listHistory(h kit.ModuleHost) []db.HistoryEntry {
	if h.DB() == nil || h.ClusterKey() == "" {
		return nil
	}
	entries, err := h.DB().ListHistory(context.Background(), h.ClusterKey(), 50, 0)
	if err != nil {
		return nil
	}
	return entries
}

func (m *Module) listSaved(h kit.ModuleHost) []db.SavedQuery {
	if h.DB() == nil || h.ClusterKey() == "" {
		return nil
	}
	queries, err := h.DB().ListQueries(context.Background(), h.ClusterKey())
	if err != nil {
		return nil
	}
	return queries
}
