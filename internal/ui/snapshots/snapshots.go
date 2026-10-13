// Package snapshots builds the「快照」module page: pick a repository, list
// its snapshots (_snapshot/<repo>/_all), create/delete/restore, snapshot
// detail, and the SLM policy section on clusters that support it.
package snapshots

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// shardInfo mirrors a snapshot's shards counts.
type shardInfo struct {
	Total      int `json:"total"`
	Failed     int `json:"failed"`
	Successful int `json:"successful"`
}

// snapEntry mirrors one entry of _snapshot/<repo>/_all.
type snapEntry struct {
	Snapshot          string    `json:"snapshot"`
	State             string    `json:"state"`
	StartTimeInMillis int64     `json:"start_time_in_millis"`
	EndTimeInMillis   int64     `json:"end_time_in_millis"`
	Indices           []string  `json:"indices"`
	Shards            shardInfo `json:"shards"`
}

// snapList is the reply of _snapshot/<repo>/_all and _snapshot/<repo>/<snap>.
type snapList struct {
	Snapshots []snapEntry `json:"snapshots"`
}

// slmPolicy mirrors the policy body inside _slm/policy/<id>.
type slmPolicy struct {
	Name       string `json:"name"`
	Schedule   string `json:"schedule"`
	Repository string `json:"repository"`
}

// slmEntry mirrors one entry of _slm/policy.
type slmEntry struct {
	Policy              slmPolicy `json:"policy"`
	NextExecutionMillis int64     `json:"next_execution_millis"`
}

// slmList is the reply of _slm/policy: id → entry.
type slmList map[string]slmEntry

// slmStatus is the reply of _slm/status.
type slmStatus struct {
	OperationMode string `json:"operation_mode"`
}

// Module holds the 快照 module's state.
type Module struct {
	repos       []string
	repoLoading bool
	reposLoaded bool
	repoErr     error
	repo        string

	snaps      []snapEntry
	loading    bool
	err        error
	loadedOnce bool
	selected   int
	prefs      kit.ColumnPrefs
	slmPrefs   kit.ColumnPrefs

	actionErr string
	actionMsg string

	// 详情面板。
	detailName string
	detail     *snapEntry
	detailBusy bool

	// 恢复对话框。
	restoreOpen              bool
	restoreName              string
	restoreIndices           string
	restorePattern           string
	restoreReplacement       string
	restoreIgnoreUnavailable bool
	restoreFocus             bool

	// 新建快照对话框。
	newOpen              bool
	newName              string
	newIndices           string
	newIgnoreUnavailable bool
	newFocus             bool

	// 自动快照（SLM）策略区。
	slmPolicies []slmEntry
	slmIDs      []string
	slmLoading  bool
	slmLoaded   bool
	slmErr      error
	slmMode     string
	slmSel      int

	// 新建自动快照策略对话框。
	slmNewOpen     bool
	slmNewName     string
	slmNewSchedule string
	slmNewRepo     string
	slmNewIndices  string
	slmNewFocus    bool
}

// New makes the module's state.
func New() *Module { return &Module{selected: -1, slmSel: -1, slmNewIndices: "*"} }

func snapCols(lang string) []ui.TableColumn {
	return []ui.TableColumn{
		{Title: kit.Tr(lang, "快照"), Sortable: true},
		{Title: kit.Tr(lang, "状态"), Width: 110},
		{Title: kit.Tr(lang, "开始"), Width: 150},
		{Title: kit.Tr(lang, "结束"), Width: 150},
		{Title: kit.Tr(lang, "索引数"), Width: 70, Align: ui.End},
		{Title: kit.Tr(lang, "成功分片"), Width: 80, Align: ui.End},
		{Title: kit.Tr(lang, "失败分片"), Width: 80, Align: ui.End},
		{Title: kit.Tr(lang, "操作"), Width: 190},
	}
}

func slmCols(lang string) []ui.TableColumn {
	return []ui.TableColumn{
		{Title: kit.Tr(lang, "策略"), Sortable: true},
		{Title: "schedule", Width: 150},
		{Title: kit.Tr(lang, "仓库"), Width: 130},
		{Title: kit.Tr(lang, "下次执行"), Width: 150},
		{Title: kit.Tr(lang, "操作"), Width: 130},
	}
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	client := h.ES()
	slmOK := es.SupportsSLM(h.Version())
	if client != nil {
		if !m.reposLoaded && !m.repoLoading {
			m.loadRepos(h)
		}
		if slmOK && !m.slmLoaded && !m.slmLoading {
			m.loadSlm(h)
		}
	}
	t := c.Theme()
	// ⌘R 刷新，系统习惯。
	c.OnShortcut(ui.Cmd, ui.KeyR, func() {
		m.loadRepos(h)
		m.loadSnaps(h)
		if slmOK {
			m.loadSlm(h)
		}
	})

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(t.Space(4), t.Space(5), t.Space(2), t.Space(5)).Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, kit.Tr(h.Lang(), "快照")).FontSize(kit.SizeTitle(t)).Bold()
				ui.Text(c, kit.Tr(h.Lang(), "{0} 个快照", len(m.snaps))).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			if len(m.repos) > 0 {
				if ui.Select(c, &m.repo, m.repos).Changed() {
					m.loadedOnce = false
					m.detailName, m.detail = "", nil
					m.loadSnaps(h)
				}
			}
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.loadRepos(h)
				m.loadSnaps(h)
				if slmOK {
					m.loadSlm(h)
				}
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "新建快照…")).Clicked() {
				m.newOpen, m.newName, m.newIndices = true, "", ""
				m.newFocus = true
			}
		})

		if m.actionErr != "" {
			ui.Text(c, m.actionErr).FontSize(kit.SizeCaption(t)).TextColor(t.Danger).Padding(0, t.Space(5), t.Space(1), t.Space(5))
		}
		if m.actionMsg != "" {
			ui.Text(c, m.actionMsg).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted).Padding(0, t.Space(5), t.Space(1), t.Space(5))
		}
		if client == nil {
			ui.Text(c, kit.Tr(h.Lang(), "未连接集群")).TextColor(t.TextMuted).Padding(0, t.Space(4))
			return
		}

		m.snapshotsTable(h, c)
		if slmOK {
			m.slmSection(h, c)
		}
		m.dialogs(h, c)
	})
}

func (m *Module) snapshotsTable(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	if kit.StateText(c, h.Lang(), m.repoLoading && !m.reposLoaded, m.repoErr, false, "") {
		return
	}
	if m.repoErr != nil {
		ui.Text(c, kit.ErrMessage(m.repoErr)).FontSize(kit.SizeCaption(t)).TextColor(t.Danger)
		return
	}
	if m.repo == "" {
		ui.Text(c, kit.Tr(h.Lang(), "先去「快照仓库」加一个仓库")).TextColor(t.TextMuted).Padding(0, t.Space(4))
		return
	}
	if m.err != nil {
		ui.Text(c, kit.ErrMessage(m.err)).FontSize(kit.SizeCaption(t)).TextColor(t.Danger).Padding(0, t.Space(4))
		return
	}
	rows := m.visibleSnaps()
	if kit.StateText(c, h.Lang(), false, nil, len(rows) == 0, kit.Tr(h.Lang(), "还没有快照")) {
		return
	}

	if m.selected >= len(rows) {
		m.selected = -1
	}
	listState := ui.ListState{Selected: &m.selected, Key: func(i int) any { return rows[i].Snapshot }, Columns: m.prefs.Frame(h, "snapshots")}
	ui.Table(c, &listState, snapCols(h.Lang()), len(rows), func(row, col int) {
		r := rows[row]
		switch col {
		case 0:
			ui.Text(c, r.Snapshot).SingleLine()
		case 1:
			ui.Row(c).Gap(t.Space(1.5)).AlignItems(ui.Center).Children(func() {
				kit.Dot(c, t.Space(1.75), stateColor(t, r.State))
				ui.Text(c, r.State)
			})
		case 2:
			ui.Text(c, fmtTime(r.StartTimeInMillis))
		case 3:
			ui.Text(c, fmtTime(r.EndTimeInMillis))
		case 4:
			ui.Text(c, strconv.Itoa(len(r.Indices)))
		case 5:
			ui.Text(c, strconv.Itoa(r.Shards.Successful))
		case 6:
			ui.Text(c, strconv.Itoa(r.Shards.Failed)).TextColor(failColor(t, r.Shards.Failed))
		case 7:
			ui.Row(c).Gap(t.Space(1.5)).Children(func() {
				if ui.Button(c, kit.Tr(h.Lang(), "详情…")).Clicked() {
					m.showDetail(h, r.Snapshot)
				}
				if ui.Button(c, kit.Tr(h.Lang(), "恢复…")).Clicked() {
					m.restoreName = r.Snapshot
					m.restoreIndices, m.restorePattern, m.restoreReplacement = "", "", ""
					m.restoreIgnoreUnavailable = false
					m.restoreOpen, m.restoreFocus = true, true
				}
				if ui.Button(c, kit.Tr(h.Lang(), "删除快照…")).Clicked() {
					m.deleteSnapshot(h, r.Snapshot)
				}
			})
		}
	}).Grow(1).Padding(0, t.Space(3), 0, t.Space(3))
	// ui.Table 在拖拽时原地改写 ListState.Columns（列变动没有专门
	// 回调），每帧把改完的布局交回 Sync 比较并持久化。
	m.prefs.Sync(h, "snapshots", listState.Columns)

	if m.detailName != "" {
		m.detailView(c, h.Lang())
	}
}

func (m *Module) detailView(c *ui.Context, lang string) {
	t := c.Theme()
	ui.Column(c).Height(t.Space(47.5)).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).
		Background(t.Surface).Padding(t.Space(3), t.Space(4), t.Space(3), t.Space(4)).Gap(t.Space(1.5)).Children(func() {
		ui.Row(c).Gap(t.Space(2)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, m.detailName).Bold()
			ui.Text(c, kit.Tr(lang, "· 详情")).TextColor(t.TextMuted).FontSize(kit.SizeCaption(t))
			ui.Spacer(c)
			if ui.Button(c, kit.Tr(lang, "关闭")).Clicked() {
				m.detailName, m.detail = "", nil
			}
		})
		if m.detailBusy {
			ui.Text(c, kit.Tr(lang, "加载中…")).TextColor(t.TextMuted)
			return
		}
		if m.detail == nil {
			return
		}
		ui.Row(c).Gap(t.Space(4)).Children(func() {
			ui.Text(c, kit.Tr(lang, "索引 {0} 个", len(m.detail.Indices)))
			ui.Text(c, kit.Tr(lang, "分片：成功 {0} · 失败 {1} · 共 {2}",
				m.detail.Shards.Successful, m.detail.Shards.Failed, m.detail.Shards.Total))
			ui.Text(c, m.detail.State).TextColor(stateColor(t, m.detail.State))
		})
		sorted := slices.Clone(m.detail.Indices)
		slices.Sort(sorted)
		ui.Text(c, strings.Join(sorted, ", ")).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
	})
}

func (m *Module) slmSection(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	ui.Column(c).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Padding(t.Space(3)).Gap(t.Space(2)).Children(func() {
		ui.Row(c).Gap(t.Space(2.5)).AlignItems(ui.Center).Children(func() {
			ui.Text(c, kit.Tr(h.Lang(), "自动快照")).Bold()
			if m.slmMode != "" {
				ui.Text(c, kit.Tr(h.Lang(), "状态：{0}", m.slmMode)).FontSize(kit.SizeCaption(t)).TextColor(t.TextMuted)
			}
			ui.Spacer(c)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.loadSlm(h)
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "新建策略…")).Clicked() {
				m.slmNewName, m.slmNewSchedule = "", ""
				m.slmNewRepo = m.repo
				if m.slmNewRepo == "" && len(m.repos) > 0 {
					m.slmNewRepo = m.repos[0]
				}
				m.slmNewOpen, m.slmNewFocus = true, true
			}
		})
		if m.slmErr != nil {
			ui.Text(c, kit.ErrMessage(m.slmErr)).FontSize(kit.SizeCaption(t)).TextColor(t.Danger)
			return
		}
		if kit.StateText(c, h.Lang(), m.slmLoading && !m.slmLoaded, nil, len(m.slmPolicies) == 0, kit.Tr(h.Lang(), "还没有策略")) {
			return
		}
		if m.slmSel >= len(m.slmPolicies) {
			m.slmSel = -1
		}
		listState := ui.ListState{Selected: &m.slmSel, Key: func(i int) any { return m.slmIDs[i] }, Columns: m.slmPrefs.Frame(h, "snapshots.slm")}
		ui.Table(c, &listState, slmCols(h.Lang()), len(m.slmPolicies), func(row, col int) {
			id, p := m.slmIDs[row], m.slmPolicies[row]
			switch col {
			case 0:
				ui.Text(c, id).SingleLine()
			case 1:
				ui.Text(c, p.Policy.Schedule)
			case 2:
				ui.Text(c, p.Policy.Repository)
			case 3:
				ui.Text(c, fmtTime(p.NextExecutionMillis))
			case 4:
				ui.Row(c).Gap(t.Space(1.5)).Children(func() {
					if ui.Button(c, kit.Tr(h.Lang(), "执行")).Clicked() {
						m.executeSlm(h, id)
					}
					if ui.Button(c, kit.Tr(h.Lang(), "删除策略…")).Clicked() {
						m.deleteSlm(h, id)
					}
				})
			}
		}).Height(180).Padding(0, 0, 0, 0)
		// ui.Table 在拖拽时原地改写 ListState.Columns（列变动没有专门
		// 回调），每帧把改完的布局交回 Sync 比较并持久化。
		m.slmPrefs.Sync(h, "snapshots.slm", listState.Columns)
	})
}

func (m *Module) dialogs(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()

	ui.Modal(c, &m.restoreOpen, func() {
		ui.Text(c, kit.Tr(h.Lang(), "恢复快照 {0}", m.restoreName)).Bold()
		ui.Form(c, func() {
			ui.Field(c, kit.Tr(h.Lang(), "索引"), func() {
				input := ui.TextInput(c, &m.restoreIndices)
				if m.restoreFocus {
					input.AutoFocus()
					m.restoreFocus = false
				}
			}).Description(kit.Tr(h.Lang(), "逗号分隔，留空恢复全部"))
			ui.Field(c, kit.Tr(h.Lang(), "改名规则"), func() { ui.TextInput(c, &m.restorePattern) })
			ui.Field(c, kit.Tr(h.Lang(), "替换为"), func() { ui.TextInput(c, &m.restoreReplacement) })
			ui.Field(c, kit.Tr(h.Lang(), "选项"), func() {
				ui.Checkbox(c, &m.restoreIgnoreUnavailable, kit.Tr(h.Lang(), "忽略不可用索引"))
			})
		})
		ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
			if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
				m.restoreOpen = false
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "恢复")).Clicked() {
				m.restore(h)
			}
		})
	})

	ui.Modal(c, &m.newOpen, func() {
		ui.Text(c, kit.Tr(h.Lang(), "新建快照（仓库 {0}）", m.repo)).Bold()
		ui.Form(c, func() {
			ui.Field(c, kit.Tr(h.Lang(), "快照名称"), func() {
				input := ui.TextInput(c, &m.newName)
				if m.newFocus {
					input.AutoFocus()
					m.newFocus = false
				}
			})
			ui.Field(c, kit.Tr(h.Lang(), "索引"), func() { ui.TextInput(c, &m.newIndices) }).Description(kit.Tr(h.Lang(), "逗号分隔，留空为全部"))
			ui.Field(c, kit.Tr(h.Lang(), "选项"), func() {
				ui.Checkbox(c, &m.newIgnoreUnavailable, kit.Tr(h.Lang(), "忽略不可用索引"))
			})
		})
		ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
			if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
				m.newOpen = false
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "创建")).Clicked() {
				m.create(h)
			}
		})
	})

	ui.Modal(c, &m.slmNewOpen, func() {
		ui.Text(c, kit.Tr(h.Lang(), "新建自动快照")).Bold()
		ui.Form(c, func() {
			ui.Field(c, kit.Tr(h.Lang(), "策略名称"), func() {
				input := ui.TextInput(c, &m.slmNewName)
				if m.slmNewFocus {
					input.AutoFocus()
					m.slmNewFocus = false
				}
			})
			ui.Field(c, kit.Tr(h.Lang(), "定时"), func() { ui.TextInput(c, &m.slmNewSchedule) }).Description(kit.Tr(h.Lang(), "cron 表达式，如 0 30 1 * * ?"))
			ui.Field(c, kit.Tr(h.Lang(), "索引"), func() { ui.TextInput(c, &m.slmNewIndices) }).Description(kit.Tr(h.Lang(), "逗号分隔，* 为全部"))
		})
		ui.Row(c).Gap(t.Space(2)).Justify(ui.End).Children(func() {
			if ui.Button(c, kit.Tr(h.Lang(), "取消")).Clicked() {
				m.slmNewOpen = false
			}
			if ui.PrimaryButton(c, kit.Tr(h.Lang(), "创建")).Clicked() {
				m.createSlm(h)
			}
		})
	})

}

// visibleSnaps returns the loaded snapshots.
func (m *Module) visibleSnaps() []snapEntry { return m.snaps }

func (m *Module) loadRepos(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	run(h, "snapshots-repos", func() { m.repoLoading = true }, func(ctx context.Context) ([]string, error) {
		var entries map[string]struct {
			Type string `json:"type"`
		}
		if _, err := client.CatRepositoriesInto(ctx, nil, &entries); err != nil {
			return nil, err
		}
		names := make([]string, 0, len(entries))
		for name := range entries {
			names = append(names, name)
		}
		slices.Sort(names)
		return names, nil
	}, func(names []string, err error) {
		m.repoLoading, m.reposLoaded = false, true
		if err != nil {
			m.repoErr = err
			return
		}
		m.repoErr, m.repos = nil, names
		if m.repo == "" && len(names) > 0 {
			m.repo = names[0]
			m.loadSnaps(h)
		}
	})
}

func (m *Module) loadSnaps(h kit.ModuleHost) {
	client := h.ES()
	if client == nil || m.repo == "" {
		return
	}
	run(h, "snapshots", func() { m.loading = true }, func(ctx context.Context) ([]snapEntry, error) {
		var list snapList
		if _, err := client.CatSnapshotsInto(ctx, m.repo, &list); err != nil {
			return nil, err
		}
		return list.Snapshots, nil
	}, func(snaps []snapEntry, err error) {
		m.loading, m.loadedOnce = false, true
		if err != nil {
			m.err = err
			return
		}
		m.err, m.snaps = nil, snaps
	})
}

func (m *Module) loadSlm(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	run3(h, "snapshots-slm", func() { m.slmLoading = true }, func(ctx context.Context) (slmList, string, error) {
		var policies slmList
		if _, err := client.CatSlmPoliciesInto(ctx, &policies); err != nil {
			return nil, "", err
		}
		mode := ""
		var s slmStatus
		if _, err := client.SlmGetStatusInto(ctx, &s); err == nil {
			mode = s.OperationMode
		}
		return policies, mode, nil
	}, func(policies slmList, mode string, err error) {
		m.slmLoading, m.slmLoaded = false, true
		if err != nil {
			m.slmErr = err
			return
		}
		m.slmErr, m.slmMode = nil, mode
		m.slmIDs = slices.Sorted(maps.Keys(policies))
		m.slmPolicies = make([]slmEntry, 0, len(m.slmIDs))
		for _, id := range m.slmIDs {
			m.slmPolicies = append(m.slmPolicies, policies[id])
		}
	})
}

func (m *Module) showDetail(h kit.ModuleHost, name string) {
	client := h.ES()
	if client == nil {
		return
	}
	m.detailName = name
	run(h, "snapshots-detail", func() { m.detailBusy = true }, func(ctx context.Context) (*snapEntry, error) {
		var list snapList
		if _, err := client.GetSnapshotInto(ctx, m.repo, name, &list); err != nil {
			return nil, err
		}
		if len(list.Snapshots) == 0 {
			return nil, fmt.Errorf("快照 %s 不存在", name)
		}
		snap := list.Snapshots[0]
		return &snap, nil
	}, func(snap *snapEntry, err error) {
		m.detailBusy = false
		if err != nil {
			m.actionErr = kit.ErrMessage(err)
			return
		}
		m.detail = snap
	})
}

func (m *Module) deleteSnapshot(h kit.ModuleHost, name string) {
	client := h.ES()
	if client == nil {
		return
	}
	run(h, "snapshots-action", func() { m.actionErr, m.actionMsg = "", "" }, func(ctx context.Context) (*es.Response, error) {
		return client.SnapshotDelete(ctx, m.repo, name)
	}, func(_ *es.Response, err error) {
		if err == nil {
			if m.detailName == name {
				m.detailName, m.detail = "", nil
			}
			m.loadSnaps(h)
			return
		}
		m.actionErr = kit.ErrMessage(err)
	})
}

func (m *Module) restore(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	body := map[string]any{}
	if indices := parseList(m.restoreIndices); len(indices) > 0 {
		body["indices"] = indices
	}
	if m.restorePattern != "" {
		body["rename_pattern"] = m.restorePattern
	}
	if m.restoreReplacement != "" {
		body["rename_replacement"] = m.restoreReplacement
	}
	if m.restoreIgnoreUnavailable {
		body["ignore_unavailable"] = true
	}
	run(h, "snapshots-action", func() { m.actionErr, m.actionMsg = "", "" }, func(ctx context.Context) (*es.Response, error) {
		return client.SnapshotRestore(ctx, m.repo, m.restoreName, body)
	}, func(_ *es.Response, err error) {
		if err == nil {
			m.restoreOpen = false
			m.actionMsg = kit.Tr(h.Lang(), "已开始恢复 {0}", m.restoreName)
			return
		}
		m.actionErr = kit.ErrMessage(err)
	})
}

func (m *Module) create(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	if m.newName == "" {
		m.actionErr = kit.Tr(h.Lang(), "快照名称不能为空")
		return
	}
	indices := parseList(m.newIndices)
	if len(indices) == 0 {
		indices = []string{"*"}
	}
	body := map[string]any{"indices": indices, "ignore_unavailable": m.newIgnoreUnavailable}
	run(h, "snapshots-action", func() { m.actionErr, m.actionMsg = "", "" }, func(ctx context.Context) (*es.Response, error) {
		return client.SnapshotCreate(ctx, m.repo, m.newName, body)
	}, func(_ *es.Response, err error) {
		if err == nil {
			m.newOpen = false
			m.loadSnaps(h)
			return
		}
		m.actionErr = kit.ErrMessage(err)
	})
}

func (m *Module) executeSlm(h kit.ModuleHost, id string) {
	client := h.ES()
	if client == nil {
		return
	}
	run(h, "snapshots-action", func() { m.actionErr, m.actionMsg = "", "" }, func(ctx context.Context) (*es.Response, error) {
		return client.SlmExecutePolicy(ctx, id)
	}, func(_ *es.Response, err error) {
		if err == nil {
			m.actionMsg = kit.Tr(h.Lang(), "已执行策略 {0}", id)
			return
		}
		m.actionErr = kit.ErrMessage(err)
	})
}

func (m *Module) deleteSlm(h kit.ModuleHost, id string) {
	client := h.ES()
	if client == nil {
		return
	}
	run(h, "snapshots-action", func() { m.actionErr, m.actionMsg = "", "" }, func(ctx context.Context) (*es.Response, error) {
		return client.SlmDeletePolicy(ctx, id)
	}, func(_ *es.Response, err error) {
		if err == nil {
			m.loadSlm(h)
			return
		}
		m.actionErr = kit.ErrMessage(err)
	})
}

func (m *Module) createSlm(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	if m.slmNewName == "" || m.slmNewSchedule == "" || m.slmNewRepo == "" {
		m.actionErr = kit.Tr(h.Lang(), "策略名、定时和仓库都要填")
		return
	}
	indices := parseList(m.slmNewIndices)
	if len(indices) == 0 {
		indices = []string{"*"}
	}
	body := map[string]any{
		"schedule":   m.slmNewSchedule,
		"repository": m.slmNewRepo,
		"config":     map[string]any{"indices": indices},
	}
	run(h, "snapshots-action", func() { m.actionErr, m.actionMsg = "", "" }, func(ctx context.Context) (*es.Response, error) {
		return client.SlmPutPolicy(ctx, m.slmNewName, body)
	}, func(_ *es.Response, err error) {
		if err == nil {
			m.slmNewOpen = false
			m.loadSlm(h)
			return
		}
		m.actionErr = kit.ErrMessage(err)
	})
}

// run wraps h.Load: fn runs off-thread, the returned closure applies its
// result on the main thread. key 是取消槽位（同 key 新加载取消旧的）。
func run[T any](h kit.ModuleHost, key string, mark func(), fn func(ctx context.Context) (T, error), apply func(T, error)) {
	h.Load(key, mark, func(ctx context.Context) func() {
		v, err := fn(ctx)
		return func() {
			apply(v, err)
		}
	})
}

func run3[A, B any](h kit.ModuleHost, key string, mark func(), fn func(ctx context.Context) (A, B, error), apply func(A, B, error)) {
	h.Load(key, mark, func(ctx context.Context) func() {
		a, b, err := fn(ctx)
		return func() {
			apply(a, b, err)
		}
	})
}

// parseList splits a comma-separated list, trimmed, empties dropped.
func parseList(s string) []string {
	var out []string
	for _, part := range strings.Split(s, ",") {
		if part = strings.TrimSpace(part); part != "" {
			out = append(out, part)
		}
	}
	return out
}

// fmtTime renders epoch millis locally, "" for zero.
func fmtTime(ms int64) string {
	if ms <= 0 {
		return ""
	}
	return time.UnixMilli(ms).Local().Format("2006-01-02 15:04:05")
}

// stateColor maps a snapshot state to the health colors.
func stateColor(t *ui.Theme, state string) ui.Color {
	switch state {
	case "SUCCESS":
		return t.Success
	case "FAILED":
		return t.Danger
	case "IN_PROGRESS", "PARTIAL":
		return t.Warning
	}
	return t.Success
}

// failColor marks failed shards red when there are any.
func failColor(t *ui.Theme, n int) ui.Color {
	if n > 0 {
		return t.Danger
	}
	return t.Success
}
