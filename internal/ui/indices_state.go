package ui

import (
	"cmp"
	"context"
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// IndicesState is the indices module's state (see GLOSSARY: Index).
type IndicesState struct {
	rows       []IndexRow
	filter     string
	hideSystem bool
	loading    bool
	err        error
	loadedOnce bool

	selectedRow int // 选中的行号，-1 为无
	sort        ui.SortOrder
	prefs       kit.ColumnPrefs
	// detail 是右侧/下钻详情（设置 JSON），空为无。
	detail        []byte
	detailName    string
	detailLoading bool

	actionErr string
}

// IndexRow is one _cat/indices row.
type IndexRow struct {
	Index        string `json:"index"`
	Health       string `json:"health"`
	Status       string `json:"status"`
	DocsCount    string `json:"docs.count"`
	StoreSize    string `json:"store.size"`
	Pri          string `json:"pri"`
	Rep          string `json:"rep"`
	CreationDate string `json:"creation.date.string"`
}

func (s *IndicesState) visibleRows() []IndexRow {
	var out []IndexRow
	for _, r := range s.rows {
		if s.hideSystem && strings.HasPrefix(r.Index, ".") {
			continue
		}
		if s.filter != "" && !strings.Contains(r.Index, s.filter) {
			continue
		}
		out = append(out, r)
	}
	slices.SortStableFunc(out, func(a, b IndexRow) int {
		var cmpRes int
		switch s.sort.Column {
		case "docs":
			cmpRes = cmp.Compare(naturalKey(a.DocsCount), naturalKey(b.DocsCount))
		case "size":
			cmpRes = cmp.Compare(naturalKey(a.StoreSize), naturalKey(b.StoreSize))
		case "health":
			cmpRes = cmp.Compare(a.Health, b.Health)
		default:
			cmpRes = strings.Compare(a.Index, b.Index)
		}
		if s.sort.Descending {
			return -cmpRes
		}
		return cmpRes
	})
	return out
}

// naturalKey turns "12,482,913"/"8.4gb" into comparable forms.
func naturalKey(s string) string {
	return strings.ReplaceAll(strings.ToLower(s), ",", "")
}

func (a *App) indicesLoad() {
	s := &a.indices
	client := a.es()
	if client == nil {
		return
	}
	a.load(
		func() { s.loading = true },
		func() func() {
			rows, err := func() ([]IndexRow, error) {
				params := map[string]any{
					"bytes": "b", "format": "json",
					"h": "index,health,status,docs.count,store.size,pri,rep,creation.date.string",
				}
				resp, err := client.CatIndices(context.Background(), params, "")
				if err != nil {
					return nil, err
				}
				var rows []IndexRow
				if err := resp.JSON(&rows); err != nil {
					return nil, err
				}
				return rows, nil
			}()
			return func() {
				s.loading = false
				s.loadedOnce = true
				s.err = err
				if err == nil {
					s.rows = rows
				}
			}
		},
	)
}

// indicesAction runs a bulk action on the selected indices, then reloads.
func (a *App) indicesAction(name, index string, fn func(ctx context.Context, client *es.Client, index string) error) {
	s := &a.indices
	client := a.es()
	if client == nil || index == "" {
		return
	}
	a.load(
		func() { s.detailLoading = true; s.actionErr = "" },
		func() func() {
			err := fn(context.Background(), client, index)
			return func() {
				s.detailLoading = false
				if err != nil {
					s.actionErr = name + " 失败: " + errMessage(err)
					return
				}
				a.indicesLoad()
			}
		},
	)
}

func (a *App) indicesShowDetail(index string) {
	s := &a.indices
	client := a.es()
	if client == nil {
		return
	}
	s.detailName = index
	a.load(
		func() { s.detailLoading = true },
		func() func() {
			resp, err := client.Request(context.Background(), "GET", esClean(index)+"/_settings", nil)
			return func() {
				s.detailLoading = false
				if err != nil {
					s.actionErr = errMessage(err)
					s.detail = nil
					return
				}
				s.detail = resp.Body
			}
		},
	)
}
