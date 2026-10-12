package ui

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"

	"seismic/internal/conn"
	"seismic/internal/db"
	"seismic/internal/ui/kit"
)

// Deps are the app's stores, injected from main.
type Deps struct {
	Conns   *conn.Store
	DB      *db.Store
	HomeDir string // user-data dir, for fallback paths
}

// AttachDeps wires the stores and reconnects the active connection.
func (a *App) AttachDeps(d Deps) {
	a.deps = d
	if c, ok := d.Conns.ActiveConnection(); ok {
		a.connectTo(c)
	}
}

// connectTo dials a saved connection and refreshes cluster state.
func (a *App) connectTo(c conn.Connection) {
	a.connecting = true
	a.invalidate()
	go func() {
		auth := a.deps.Conns.ESAuth(c)
		client, info, err := conn.TestConnection(a.backgroundCtx(), c.URI, auth)
		var health string
		if err == nil {
			client.Distribution = info.Distribution
			client.BuildFlavor = info.BuildFlavor
			hresp, herr := client.Request(a.backgroundCtx(), "GET", "_cluster/health", nil)
			if herr == nil {
				var h struct {
					Status string `json:"status"`
				}
				if jsonErr := hresp.JSON(&h); jsonErr == nil {
					health = h.Status
				}
			}
		}
		a.update(func() {
			a.connecting = false
			if err != nil {
				a.Connected = false
				a.ClusterHealth = "unknown"
				a.connect.testMsg = err.Error()
				return
			}
			a.Connected = true
			a.client = client
			a.clusterUUID = info.UUID
			a.ClusterName = pick(c.Name, info.ClusterName)
			a.DistributionName = info.Distribution
			a.ClusterVer = info.Version
			a.ClusterHealth = health
			a.UpdatedText = kit.Tr(a.lang, "刚刚更新")
			go a.refreshHealth() // 立即填充状态条的文档/分片/存储
		})
		time.AfterFunc(30*time.Second, a.refreshHealth)
	}()
}

func pick(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

// refreshHealth polls cluster health for the status bar.
func (a *App) refreshHealth() {
	if !a.Connected || a.client == nil {
		return
	}
	client := a.client
	go func() {
		ctx, cancel := context.WithTimeout(a.backgroundCtx(), 10*time.Second)
		defer cancel()
		resp, err := client.Request(ctx, "GET", "_cluster/health", nil)
		if err != nil {
			return
		}
		var h struct {
			Status           string `json:"status"`
			UnassignedShards int    `json:"unassigned_shards"`
		}
		if err := resp.JSON(&h); err != nil || h.Status == "" {
			return
		}
		var docs int64
		var shardsTotal, shardPri int
		var size int64
		if sresp, serr := client.Request(ctx, "GET", "_cluster/stats", nil); serr == nil {
			var s struct {
				Indices struct {
					Docs struct {
						Count int64 `json:"count"`
					} `json:"docs"`
					Shards struct {
						Total     int `json:"total"`
						Primaries int `json:"primaries"`
					} `json:"shards"`
					Store struct {
						SizeInBytes int64 `json:"size_in_bytes"`
					} `json:"store"`
				} `json:"indices"`
			}
			if jerr := sresp.JSON(&s); jerr == nil {
				docs, shardsTotal, shardPri, size = s.Indices.Docs.Count, s.Indices.Shards.Total, s.Indices.Shards.Primaries, s.Indices.Store.SizeInBytes
			}
		}
		a.update(func() {
			if a.client != client {
				return
			}
			a.ClusterHealth = h.Status
			a.DocsCount = formatCount(docs)
			if shardsTotal > 0 {
				a.ShardsText = kit.Tr(a.lang, "{0} 分片 · {1} 主 · {2} 未分配", shardsTotal, shardPri, h.UnassignedShards)
			}
			if size > 0 {
				a.StoreText = humanSize(size)
			}
			a.UpdatedText = kit.Tr(a.lang, "刚刚更新")
		})
		time.AfterFunc(30*time.Second, a.refreshHealth)
	}()
}

// saveConnection persists the form as a connection and connects to it.
func (a *App) saveConnection() {
	f := &a.connect
	c := conn.Connection{
		Name:     pick(f.Name, f.URI),
		URI:      f.URI,
		AuthType: f.AuthType,
		Username: f.Username,
		Region:   f.Region,
	}
	creds := conn.Credentials{
		Password:        f.Password,
		APIKey:          f.APIKey,
		AccessKeyID:     f.AccessKeyID,
		SecretAccessKey: f.SecretAccessKey,
		SessionToken:    f.SessionToken,
	}
	uuid, err := a.deps.Conns.Add(c, creds)
	if err != nil {
		f.tested = true
		f.testOK = false
		f.testMsg = err.Error()
		a.invalidate()
		return
	}
	saved, _ := a.deps.Conns.Get(uuid)
	a.connectTo(saved)
}

// formatCount renders 178199002 as "1.78 亿" style short text for the bar.
func formatCount(n int64) string {
	switch {
	case n >= 100_000_000:
		return fmt.Sprintf("%.2f 亿", float64(n)/100_000_000)
	case n >= 10_000:
		return fmt.Sprintf("%.1f 万", float64(n)/10_000)
	default:
		return fmt.Sprintf("%d", n)
	}
}

func humanSize(bytes int64) string {
	const unit = 1024.0
	b := float64(bytes)
	for _, s := range []string{"B", "KB", "MB", "GB", "TB"} {
		if b < unit {
			return fmt.Sprintf("%.1f %s", b, s)
		}
		b /= unit
	}
	return fmt.Sprintf("%.1f PB", b)
}

// healthDotColor maps a cluster status to the app's health color.
func healthDotColor(c *ui.Context, health string) ui.Color {
	return healthColor(c, health)
}
