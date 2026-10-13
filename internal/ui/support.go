package ui

import (
	"context"
	"errors"

	"github.com/egoist/mygo/ui"

	"seismic/internal/es"
	"seismic/internal/ui/kit"
)

// es returns the connected cluster's client, nil when not connected.
func (a *App) es() *es.Client { return a.client }

// clusterKey returns the connected cluster's UUID, the partition key for
// per-cluster data (REST history, table prefs); "" when not connected.
func (a *App) clusterKey() string { return a.clusterUUID }

// loadSlot 是一个在途加载的取消句柄；指针身份用于识别「还是不是最新
// 一轮」（函数值不能直接比较）。
type loadSlot struct {
	cancel context.CancelFunc
}

// load runs fn in a goroutine and applies what it returns on the main
// thread: markLoading flips the module's loading flag, fn does the request
// work off-thread and returns the state-apply closure. 同 key 的新加载会
// 取消在途的旧加载（重查询不堆积）；切连接时 cancelAllLoads 全部取消。
// 都在主线程读写，worker 只持有自己的 ctx。
func (a *App) load(key string, markLoading func(), fn func(context.Context) func()) {
	ctx, cancel := context.WithCancel(a.backgroundCtx())
	slot := &loadSlot{cancel: cancel}
	a.loadsMu.Lock()
	if a.loads == nil {
		a.loads = map[string]*loadSlot{}
	}
	if old := a.loads[key]; old != nil {
		old.cancel()
	}
	a.loads[key] = slot
	a.loadsMu.Unlock()
	markLoading()
	a.invalidate()
	go func() {
		apply := fn(ctx)
		a.update(func() {
			a.loadsMu.Lock()
			cur, ok := a.loads[key]
			a.loadsMu.Unlock()
			if ok && cur != slot {
				return // 已被同 key 的新一轮取代：丢弃结果，等新的
			}
			apply()
		})
	}()
}

// cancelAllLoads cancels every in-flight load (cluster switch); the
// cancelled workers still apply, so modules land in the 已取消 state
// instead of spinning forever.
func (a *App) cancelAllLoads() {
	a.loadsMu.Lock()
	for _, slot := range a.loads {
		slot.cancel()
	}
	a.loads = nil
	a.loadsMu.Unlock()
}

// errMessage flattens an error for display, surfacing the ES error body.
func errMessage(err error) string {
	if err == nil {
		return ""
	}
	if httpErr, ok := err.(*es.HTTPError); ok {
		return httpErr.Error()
	}
	return err.Error()
}

// loadingText renders a module's loading/error/empty state row; it returns
// true when the module should not build its content this frame.
func stateText(c *ui.Context, lang string, loading bool, err error, empty bool, emptyText string) bool {
	t := c.Theme()
	switch {
	case loading:
		ui.Text(c, kit.Tr(lang, "加载中…")).TextColor(t.TextMuted)
		return true
	case err != nil:
		if errors.Is(err, context.Canceled) {
			ui.Text(c, kit.Tr(lang, "已取消（切换了连接或被新查询取代）")).TextColor(t.TextMuted)
			return true
		}
		ui.Column(c).Grow(1).Padding(16).Children(func() {
			ui.Text(c, kit.Tr(lang, "请求失败")).Bold().TextColor(ui.Hex("#d64545"))
			ui.Text(c, errMessage(err)).TextColor(t.TextMuted).FontSize(12)
		})
		return true
	case empty:
		ui.Text(c, emptyText).TextColor(t.TextMuted)
		return true
	}
	return false
}
