package matching

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// TestEngineRun 检查 Engine.Run 能通过 channel 接收命令并返回结果:
//
//	启动 Run goroutine
//	发送一个 PlaceOrder 命令(先挂一笔卖单, 再提交买单)
//	断言通过 Reply channel 收到正确的 Trade
//
// aha: 撮合代码(book.Submit)原封不动, Engine 只是在外面包了一层 channel 信封。
func TestEngineRun(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 在独立 goroutine 启动引擎
	go e.Run(ctx)

	// 先挂一笔卖单
	reply1 := make(chan Result, 1)
	e.cmds <- Command{
		Op:    OpPlaceOrder,
		Order: Order{ID: 1, Side: Sell, Type: Limit, Price: 100, Qty: 10},
		Reply: reply1,
	}

	// 等待回复, 应该零成交(只是挂单)
	select {
	case r := <-reply1:
		if r.Error != nil {
			t.Fatalf("挂卖单报错: %v", r.Error)
		}
		if len(r.Trades) != 0 {
			t.Errorf("挂单应零成交, got %d trades", len(r.Trades))
		}
	case <-time.After(time.Second):
		t.Fatal("挂卖单超时, Run 可能没有处理命令")
	}

	// 再提交一笔买单, 应该跟卖单成交
	reply2 := make(chan Result, 1)
	e.cmds <- Command{
		Op:    OpPlaceOrder,
		Order: Order{ID: 2, Side: Buy, Type: Limit, Price: 100, Qty: 10},
		Reply: reply2,
	}

	select {
	case r := <-reply2:
		if r.Error != nil {
			t.Fatalf("提交买单报错: %v", r.Error)
		}
		if len(r.Trades) != 1 {
			t.Fatalf("应恰一笔成交, got %d", len(r.Trades))
		}
		if r.Trades[0].Price != 100 || r.Trades[0].Qty != 10 {
			t.Errorf("Trade = {Price:%d, Qty:%d}, 期望 {100, 10}",
				r.Trades[0].Price, r.Trades[0].Qty)
		}
	case <-time.After(time.Second):
		t.Fatal("提交买单超时")
	}

	// 撤一个不存在的单
	reply3 := make(chan Result, 1)
	e.cmds <- Command{
		Op:      OpCancelOrder,
		OrderID: 999,
		Reply:   reply3,
	}

	select {
	case r := <-reply3:
		if r.Error == nil {
			t.Error("撤不存在的单应返回 error")
		}
	case <-time.After(time.Second):
		t.Fatal("撤单超时")
	}
}

// TestEngineRunStopsOnCancel 检查取消 context 后 Run 会退出。
func TestEngineRunStopsOnCancel(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan struct{})
	go func() {
		e.Run(ctx)
		close(done) // Run 返回后关闭
	}()

	cancel() // 取消 context

	select {
	case <-done:
		// Run 正常退出 ✅
	case <-time.After(time.Second):
		t.Fatal("取消 context 后 Run 应该退出, 但超时了")
	}
}

// TestPlaceRoundtrip 检查 Place 方法: 正常提交订单并收到成交结果。
func TestPlaceRoundtrip(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	// 先挂一笔卖单
	_, err := e.Place(ctx, Order{ID: 1, Side: Sell, Type: Limit, Price: 100, Qty: 10})
	if err != nil {
		t.Fatalf("挂卖单报错: %v", err)
	}

	// 再提交买单, 应成交
	trades, err := e.Place(ctx, Order{ID: 2, Side: Buy, Type: Limit, Price: 100, Qty: 10})
	if err != nil {
		t.Fatalf("提交买单报错: %v", err)
	}
	if len(trades) != 1 || trades[0].Qty != 10 {
		t.Errorf("trades = %+v, 期望 1 笔 Qty=10 的成交", trades)
	}
}

// TestPlaceContextCancelled 检查: 用已取消的 ctx 调 Place, 应立刻返回 ctx.Err(), 不卡死。
func TestPlaceContextCancelled(t *testing.T) {
	e := NewEngine()
	runCtx, runCancel := context.WithCancel(context.Background())
	defer runCancel()
	go e.Run(runCtx)

	// 创建一个已经取消的 ctx
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel() // 立刻取消

	_, err := e.Place(cancelledCtx, Order{ID: 1, Side: Buy, Type: Limit, Price: 100, Qty: 10})
	if err != cancelledCtx.Err() {
		t.Errorf("Place(cancelledCtx) = %v, 期望 %v", err, cancelledCtx.Err())
	}
}

// TestCancelOrder 检查 Cancel 方法: 撤一笔已存在的单和不存在的单。
func TestCancelOrderViaEngine(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	// 先挂一笔
	e.Place(ctx, Order{ID: 1, Side: Sell, Type: Limit, Price: 100, Qty: 10})

	// 撤掉
	err := e.Cancel(ctx, 1)
	if err != nil {
		t.Errorf("Cancel(1) 报错: %v", err)
	}

	// 撤不存在的
	err = e.Cancel(ctx, 999)
	if err == nil {
		t.Error("Cancel(999) 应返回 error")
	}
}

// TestConcurrent 检查并发安全:
// 100 个 goroutine 同时向一个 Engine 交错 place/cancel, 不 panic、不 race。
//
// 运行方式: go test -race ./matching/ -run TestConcurrent
//
// aha: 如果设计对了(单写 channel), 这个测试不需要写任何新代码就能过。
// 红了说明你哪里引入了共享状态。
func TestConcurrent(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	const n = 100
	done := make(chan struct{}, n)

	for i := 0; i < n; i++ {
		go func(id int) {
			defer func() { done <- struct{}{} }()

			orderID := int64(id + 1)

			// 下一笔限价单
			_, _ = e.Place(ctx, Order{
				ID:    orderID,
				Side:  Buy,
				Type:  Limit,
				Price: 100,
				Qty:   5,
			})

			// 尝试撤掉(可能已经成交了, 忽略 error)
			_ = e.Cancel(ctx, orderID)

			// 再下一笔卖单
			_, _ = e.Place(ctx, Order{
				ID:    orderID + 1000,
				Side:  Sell,
				Type:  Limit,
				Price: 100,
				Qty:   3,
			})
		}(i)
	}

	// 等所有 goroutine 完成
	for i := 0; i < n; i++ {
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatal("超时, 可能死锁了")
		}
	}
}

// TestSnapshotIsolation 检查快照隔离性:
//
//	取 S1 → 下一笔新单 → 取 S2
//	S1 不应该被新单影响(深拷贝, 不共享底层数组)
//	S2 应该反映新单
func TestSnapshotIsolation(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	// 挂一笔卖单
	e.Place(ctx, Order{ID: 1, Side: Sell, Type: Limit, Price: 100, Qty: 10})

	// 取快照 S1
	s1, err := e.GetSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetSnapshot s1 报错: %v", err)
	}
	if len(s1.Asks) != 1 {
		t.Fatalf("s1.Asks 长度 = %d, 期望 1", len(s1.Asks))
	}

	// 再挂一笔卖单(不同价格)
	e.Place(ctx, Order{ID: 2, Side: Sell, Type: Limit, Price: 105, Qty: 5})

	// 取快照 S2
	s2, err := e.GetSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetSnapshot s2 报错: %v", err)
	}

	// S1 不应该变(隔离性): 还是只有 1 个卖方档位
	if len(s1.Asks) != 1 {
		t.Errorf("s1 被污染了! s1.Asks 长度 = %d, 期望 1", len(s1.Asks))
	}

	// S2 应该有 2 个卖方档位
	if len(s2.Asks) != 2 {
		t.Errorf("s2.Asks 长度 = %d, 期望 2", len(s2.Asks))
	}
}

// TestGracefulShutdown 检查完整的启动-流量-关停流程:
//
//	启动 Engine + HTTP 服务
//	发送几笔交易请求
//	关停 HTTP 服务 + 取消引擎 context
//	断言 Run 退出、无挂死
//
// aha: 先关 HTTP(停止接受新请求), 再取消 Engine context(Run 返回)。
// 绝不 close(cmds), 因为 close 后如果有 goroutine 还在往里发命令会 panic。
func TestGracefulShutdown(t *testing.T) {
	// 1. 启动引擎
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())

	engineDone := make(chan struct{})
	go func() {
		e.Run(ctx)
		close(engineDone)
	}()

	// 2. 启动 HTTP 服务
	router := SetupRouter(e)
	srv := httptest.NewServer(router)

	// 3. 发送一些请求
	postOrder := func(id int64, side string, price, qty int64) {
		body, _ := json.Marshal(PlaceOrderRequest{
			ID: id, Side: side, Type: "limit", Price: price, Qty: qty,
		})
		resp, err := http.Post(srv.URL+"/orders", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Errorf("POST /orders 失败: %v", err)
			return
		}
		resp.Body.Close()
	}

	postOrder(1, "sell", 100, 10)
	postOrder(2, "sell", 101, 5)
	postOrder(3, "buy", 100, 10) // 应跟 ID=1 成交

	// 验证盘口
	resp, err := http.Get(srv.URL + "/book")
	if err != nil {
		t.Fatalf("GET /book 失败: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("GET /book status = %d, 期望 200", resp.StatusCode)
	}

	// 4. 优雅关停: 先关 HTTP, 再关引擎
	srv.Close()     // 关 HTTP
	cancel()        // 取消 Engine context → Run 退出

	// 5. 断言 Run 退出, 不挂死
	select {
	case <-engineDone:
		// ✅ Run 正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("Run 应该在 context 取消后退出, 但超时了")
	}
}
