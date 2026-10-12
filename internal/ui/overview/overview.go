// Package overview builds the「概览」module page.
package overview

import (
	"context"
	"fmt"
	"strconv"

	"github.com/egoist/mygo/ui"

	"seismic/internal/ui/kit"
)

// Module holds the 概览 module's state.
type Module struct {
	loading    bool
	err        error
	loadedOnce bool

	info   clusterInfo
	health clusterHealth
	stats  clusterStats
}

// New makes the module's state.
func New() *Module { return &Module{} }

type clusterInfo struct {
	ClusterName string `json:"cluster_name"`
	ClusterUUID string `json:"cluster_uuid"`
	Version     struct {
		Number       string `json:"number"`
		Distribution string `json:"distribution"`
	} `json:"version"`
}

type clusterHealth struct {
	Status           string  `json:"status"`
	ActiveShards     float64 `json:"active_shards"`
	UnassignedShards float64 `json:"unassigned_shards"`
	NumberOfNodes    float64 `json:"number_of_nodes"`
}

type clusterStats struct {
	Status string `json:"status"`
	Nodes  struct {
		Count struct {
			Total  float64 `json:"total"`
			Master float64 `json:"master"`
			Data   float64 `json:"data"`
		} `json:"count"`
	} `json:"nodes"`
	Indices struct {
		Count  float64 `json:"count"`
		Shards struct {
			Total     float64 `json:"total"`
			Primaries float64 `json:"primaries"`
		} `json:"shards"`
		Docs struct {
			Count float64 `json:"count"`
		} `json:"docs"`
		Store struct {
			SizeInBytes float64 `json:"size_in_bytes"`
		} `json:"store"`
	} `json:"indices"`
}

// load fetches cluster info, health and stats off-thread.
func (m *Module) load(h kit.ModuleHost) {
	client := h.ES()
	if client == nil {
		return
	}
	h.Load(
		func() { m.loading = true },
		func() func() {
			ctx := context.Background()
			var info clusterInfo
			var health clusterHealth
			var stats clusterStats
			var err error
			if resp, e := client.ClusterInfo(ctx); e == nil {
				_ = resp.JSON(&info)
			}
			if resp, e := client.ClusterHealth(ctx); e == nil {
				_ = resp.JSON(&health)
			} else {
				err = e
			}
			if resp, e := client.ClusterStats(ctx); e == nil {
				_ = resp.JSON(&stats)
			} else if err == nil {
				err = e
			}
			return func() {
				m.loading = false
				m.loadedOnce = true
				m.err = err
				if err == nil {
					m.info = info
					m.health = health
					m.stats = stats
				}
			}
		},
	)
}

func fmtInt(f float64) string { return strconv.FormatFloat(f, 'f', -1, 64) }

// humanBytes renders a byte count the way the platform tools do.
func humanBytes(b float64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%.0f B", b)
	}
	div, exp := float64(unit), 0
	for n := b / unit; n >= unit && exp < 5; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", b/div, "KMGTPE"[exp])
}

// View builds the page; h is the app host.
func (m *Module) View(h kit.ModuleHost, c *ui.Context) {
	t := c.Theme()
	if !m.loadedOnce && !m.loading && h.ES() != nil {
		m.load(h)
	}

	ui.Column(c).Fill().Children(func() {
		ui.Row(c).Padding(16, 20, 8, 20).Gap(10).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Children(func() {
				ui.Text(c, kit.Tr(h.Lang(), "概览")).FontSize(19).Bold()
				ui.Text(c, kit.Tr(h.Lang(), "集群状态一览")).FontSize(12).TextColor(t.TextMuted)
			})
			ui.Spacer(c)
			if ui.Button(c, kit.Tr(h.Lang(), "刷新")).Clicked() {
				m.load(h)
			}
		})

		if kit.StateText(c, h.Lang(), m.loading, m.err, false, "") {
			return
		}

		ui.Row(c).Grow(1).Padding(0, 16, 16, 16).Gap(12).Children(func() {
			m.clusterCard(c, h)
			m.healthCard(c, h.Lang())
			m.statusCard(c, h.Lang())
			m.dataCard(c, h.Lang())
		})
	})
}

func (m *Module) clusterCard(c *ui.Context, h kit.ModuleHost) {
	t := c.Theme()
	version := m.info.Version.Number
	if version == "" {
		version = h.Version()
	}
	distribution := m.info.Version.Distribution
	if distribution == "" {
		distribution = h.Distribution()
	}
	ui.Column(c).Grow(1).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Padding(14).Gap(6).Children(func() {
		ui.Text(c, kit.Tr(h.Lang(), "集群")).FontSize(12).TextColor(t.TextMuted)
		ui.Text(c, kit.Tr(h.Lang(), orDefault(m.info.ClusterName, "未知集群"))).FontSize(17).Bold()
		ui.Text(c, m.info.ClusterUUID).FontSize(11).TextColor(t.TextMuted)
		ui.Text(c, version).FontSize(12)
		if distribution != "" {
			ui.Text(c, distribution).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (m *Module) healthCard(c *ui.Context, lang string) {
	t := c.Theme()
	status := orDefault(m.health.Status, m.stats.Status)
	ui.Column(c).Grow(1).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Padding(14).Gap(6).Children(func() {
		ui.Text(c, kit.Tr(lang, "健康")).FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(10).AlignItems(ui.Center).Children(func() {
			kit.Dot(c, 16, kit.HealthColor(c, status))
			ui.Text(c, kit.Tr(lang, orDefault(status, "未知"))).FontSize(26).Bold().TextColor(kit.HealthColor(c, status))
		})
		ui.Text(c, kit.Tr(lang, "活动分片 {0}", fmtInt(m.health.ActiveShards))).FontSize(12).TextColor(t.TextMuted)
		ui.Text(c, kit.Tr(lang, "未分配分片 {0}", fmtInt(m.health.UnassignedShards))).FontSize(12).TextColor(t.TextMuted)
	})
}

func (m *Module) statusCard(c *ui.Context, lang string) {
	t := c.Theme()
	nodes := m.stats.Nodes.Count.Total
	if nodes == 0 {
		nodes = m.health.NumberOfNodes
	}
	ui.Column(c).Grow(1).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Padding(14).Gap(6).Children(func() {
		ui.Text(c, kit.Tr(lang, "状态")).FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmtInt(nodes)).FontSize(26).Bold()
			ui.Text(c, kit.Tr(lang, "节点")).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Text(c, fmtInt(m.stats.Nodes.Count.Master)+" master · "+fmtInt(m.stats.Nodes.Count.Data)+" data").FontSize(12).TextColor(t.TextMuted)
		ui.Text(c, kit.Tr(lang, "{0} 主分片 · {1} 副本", fmtInt(m.stats.Indices.Shards.Primaries), fmtInt(m.stats.Indices.Shards.Total-m.stats.Indices.Shards.Primaries))).FontSize(12).TextColor(t.TextMuted)
	})
}

func (m *Module) dataCard(c *ui.Context, lang string) {
	t := c.Theme()
	ui.Column(c).Grow(1).BorderWidth(1, 1, 1, 1).BorderColor(t.Border).Padding(14).Gap(6).Children(func() {
		ui.Text(c, kit.Tr(lang, "数据")).FontSize(12).TextColor(t.TextMuted)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, fmtInt(m.stats.Indices.Docs.Count)).FontSize(26).Bold()
			ui.Text(c, kit.Tr(lang, "文档")).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Text(c, kit.Tr(lang, "{0} 磁盘占用", humanBytes(m.stats.Indices.Store.SizeInBytes))).FontSize(12).TextColor(t.TextMuted)
		ui.Text(c, kit.Tr(lang, "{0} 个索引", fmtInt(m.stats.Indices.Count))).FontSize(12).TextColor(t.TextMuted)
	})
}

func orDefault(s, fallback string) string {
	if s == "" {
		return fallback
	}
	return s
}
