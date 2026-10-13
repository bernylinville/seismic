package ui

import (
	"context"

	"seismic/internal/es"
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
