package ui

import (
	"context"
	"sync"
	"testing"
	"time"
)

// TestLoadSupersededResultDropped 同 key 重查询：旧一轮被取消且结果丢弃，
// 只有最新一轮的 apply 落地。
func TestLoadSupersededResultDropped(t *testing.T) {
	a := &App{}
	started := make(chan struct{})
	newApplied := make(chan struct{})
	var mu sync.Mutex
	var applied []string

	a.load("k", func() {}, func(ctx context.Context) func() {
		close(started)
		<-ctx.Done() // 模拟在途请求，直到被取消
		return func() {
			mu.Lock()
			applied = append(applied, "old")
			mu.Unlock()
		}
	})
	<-started

	a.load("k", func() {}, func(ctx context.Context) func() {
		return func() {
			mu.Lock()
			applied = append(applied, "new")
			mu.Unlock()
			select {
			case <-newApplied:
			default:
				close(newApplied)
			}
		}
	})

	select {
	case <-newApplied:
	case <-time.After(2 * time.Second):
		t.Fatal("新一轮加载没有完成")
	}
	time.Sleep(100 * time.Millisecond) // 给被取消的旧 worker 收尾时间

	mu.Lock()
	defer mu.Unlock()
	if len(applied) != 1 || applied[0] != "new" {
		t.Fatalf("applied = %v, want only the superseding load's result", applied)
	}
}

// TestCancelAllLoads 切连接：全部在途加载被取消，apply 仍执行（模块能清掉
// loading 态并显示已取消）。
func TestCancelAllLoads(t *testing.T) {
	a := &App{}
	started := make(chan struct{})
	applied := make(chan struct{})

	a.load("k", func() {}, func(ctx context.Context) func() {
		close(started)
		<-ctx.Done()
		return func() { close(applied) }
	})
	<-started

	a.cancelAllLoads()
	select {
	case <-applied:
	case <-time.After(2 * time.Second):
		t.Fatal("cancelAllLoads 没有取消在途加载")
	}
	if a.loads != nil {
		t.Error("cancelAllLoads 应清空注册表")
	}
}
