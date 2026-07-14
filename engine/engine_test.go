package engine

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"exchange/eventbus"
	"exchange/ledger"
	"exchange/matching"
	"exchange/store"

	"github.com/gorilla/websocket"
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
		Order: matching.Order{ID: 1, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10},
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
		Order: matching.Order{ID: 2, Side: matching.Buy, Type: matching.Limit, Price: 100, Qty: 10},
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
	_, err := e.Place(ctx, matching.Order{ID: 1, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10})
	if err != nil {
		t.Fatalf("挂卖单报错: %v", err)
	}

	// 再提交买单, 应成交
	trades, err := e.Place(ctx, matching.Order{ID: 2, Side: matching.Buy, Type: matching.Limit, Price: 100, Qty: 10})
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

	_, err := e.Place(cancelledCtx, matching.Order{ID: 1, Side: matching.Buy, Type: matching.Limit, Price: 100, Qty: 10})
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
	e.Place(ctx, matching.Order{ID: 1, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10})

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
// 运行方式: go test -race ./engine/ -run TestConcurrent
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
			_, _ = e.Place(ctx, matching.Order{
				ID:    orderID,
				Side:  matching.Buy,
				Type:  matching.Limit,
				Price: 100,
				Qty:   5,
			})

			// 尝试撤掉(可能已经成交了, 忽略 error)
			_ = e.Cancel(ctx, orderID)

			// 再下一笔卖单
			_, _ = e.Place(ctx, matching.Order{
				ID:    orderID + 1000,
				Side:  matching.Sell,
				Type:  matching.Limit,
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
	e.Place(ctx, matching.Order{ID: 1, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10})

	// 取快照 S1
	s1, err := e.GetSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetSnapshot s1 报错: %v", err)
	}
	if len(s1.Asks) != 1 {
		t.Fatalf("s1.Asks 长度 = %d, 期望 1", len(s1.Asks))
	}

	// 再挂一笔卖单(不同价格)
	e.Place(ctx, matching.Order{ID: 2, Side: matching.Sell, Type: matching.Limit, Price: 105, Qty: 5})

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
	srv.Close() // 关 HTTP
	cancel()    // 取消 Engine context → Run 退出

	// 5. 断言 Run 退出, 不挂死
	select {
	case <-engineDone:
		// ✅ Run 正常退出
	case <-time.After(2 * time.Second):
		t.Fatal("Run 应该在 context 取消后退出, 但超时了")
	}
}

// TestPostOrders 检查 POST /orders 下单接口。
func TestPostOrders(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	r := SetupRouter(e)

	// 先挂一笔卖单
	body1, _ := json.Marshal(PlaceOrderRequest{
		ID: 1, Side: "sell", Type: "limit", Price: 100, Qty: 10,
	})
	w1 := httptest.NewRecorder()
	req1, _ := http.NewRequest("POST", "/orders", bytes.NewReader(body1))
	req1.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w1, req1)

	if w1.Code != http.StatusCreated {
		t.Errorf("挂卖单: status = %d, 期望 201", w1.Code)
	}

	// 再提交买单, 应成交
	body2, _ := json.Marshal(PlaceOrderRequest{
		ID: 2, Side: "buy", Type: "limit", Price: 100, Qty: 10,
	})
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("POST", "/orders", bytes.NewReader(body2))
	req2.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusCreated {
		t.Errorf("提交买单: status = %d, 期望 201", w2.Code)
	}

	// 解析响应, 应有成交
	var resp struct {
		Trades []matching.Trade `json:"trades"`
	}
	if err := json.Unmarshal(w2.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析响应 JSON 失败: %v", err)
	}
	if len(resp.Trades) != 1 {
		t.Errorf("trades 数量 = %d, 期望 1", len(resp.Trades))
	}
}

// TestGetBook 检查 GET /book 快照接口。
func TestGetBook(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	r := SetupRouter(e)

	// 先挂两笔单
	for _, req := range []PlaceOrderRequest{
		{ID: 1, Side: "sell", Type: "limit", Price: 100, Qty: 10},
		{ID: 2, Side: "buy", Type: "limit", Price: 95, Qty: 5},
	} {
		body, _ := json.Marshal(req)
		w := httptest.NewRecorder()
		httpReq, _ := http.NewRequest("POST", "/orders", bytes.NewReader(body))
		httpReq.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, httpReq)
	}

	// GET /book
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("GET", "/book", nil)
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("GET /book: status = %d, 期望 200", w.Code)
	}

	var snap matching.BookSnapshot
	if err := json.Unmarshal(w.Body.Bytes(), &snap); err != nil {
		t.Fatalf("解析快照 JSON 失败: %v", err)
	}
	if len(snap.Asks) != 1 {
		t.Errorf("Asks 数量 = %d, 期望 1", len(snap.Asks))
	}
	if len(snap.Bids) != 1 {
		t.Errorf("Bids 数量 = %d, 期望 1", len(snap.Bids))
	}
}

// TestDeleteOrders 检查 DELETE /orders/:id 撤单接口。
func TestDeleteOrders(t *testing.T) {
	e := NewEngine()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	r := SetupRouter(e)

	// 先挂一笔
	body, _ := json.Marshal(PlaceOrderRequest{
		ID: 1, Side: "sell", Type: "limit", Price: 100, Qty: 10,
	})
	w := httptest.NewRecorder()
	req, _ := http.NewRequest("POST", "/orders", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	// 撤掉
	w2 := httptest.NewRecorder()
	req2, _ := http.NewRequest("DELETE", "/orders/1", nil)
	r.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("DELETE /orders/1: status = %d, 期望 200", w2.Code)
	}

	// 二次删 → 404
	w3 := httptest.NewRecorder()
	req3, _ := http.NewRequest("DELETE", "/orders/1", nil)
	r.ServeHTTP(w3, req3)

	if w3.Code != http.StatusNotFound {
		t.Errorf("二次 DELETE: status = %d, 期望 404", w3.Code)
	}
}

// TestWebSocket 检查 WebSocket 连接能收到成交事件。
func TestWebSocket(t *testing.T) {
	bus := eventbus.NewEventBus()
	e := NewEngine()
	e.SetEventBus(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	router := SetupRouterWithBus(e, bus)
	srv := httptest.NewServer(router)
	defer srv.Close()

	// 连接 WebSocket
	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("WebSocket 连接失败: %v", err)
	}
	defer ws.Close()

	// 挂卖单 + 买单成交
	e.Place(ctx, matching.Order{ID: 1, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10})
	e.Place(ctx, matching.Order{ID: 2, Side: matching.Buy, Type: matching.Limit, Price: 100, Qty: 10})

	// 应收到事件
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("读 WebSocket 消息失败: %v", err)
	}

	var event eventbus.Event
	if err := json.Unmarshal(msg, &event); err != nil {
		t.Fatalf("解析事件 JSON 失败: %v", err)
	}

	if event.Type != "trade" && event.Type != "book_update" {
		t.Errorf("事件类型 = %q, 期望 trade 或 book_update", event.Type)
	}
}

// TestRouterTwoPairs 检查两个币对各自下单互不干扰。
func TestRouterTwoPairs(t *testing.T) {
	l := ledger.NewLedger()
	r := NewRouter(l)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eth := matching.Symbol{Base: "ETH", Quote: "USD"}
	btc := matching.Symbol{Base: "BTC", Quote: "USD"}
	r.Register(ctx, eth)
	r.Register(ctx, btc)

	// 给用户入金
	l.Deposit(1, "USD", 100000)
	l.Deposit(2, "ETH", 100)
	l.Deposit(3, "BTC", 50)

	// ETH/USD: 用户 2 挂卖 10 ETH @ 100
	r.Place(ctx, eth, matching.Order{ID: 1, OwnerID: 2, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10})
	// 用户 1 买 10 ETH @ 100
	trades, err := r.Place(ctx, eth, matching.Order{ID: 2, OwnerID: 1, Side: matching.Buy, Type: matching.Limit, Price: 100, Qty: 10})
	if err != nil {
		t.Fatalf("ETH/USD 下单报错: %v", err)
	}
	if len(trades) != 1 || trades[0].Qty != 10 {
		t.Errorf("ETH/USD 应成交 10, got %+v", trades)
	}

	// BTC/USD: 用户 3 挂卖 5 BTC @ 5000
	r.Place(ctx, btc, matching.Order{ID: 3, OwnerID: 3, Side: matching.Sell, Type: matching.Limit, Price: 5000, Qty: 5})
	// 用户 1 买 5 BTC @ 5000
	trades2, err := r.Place(ctx, btc, matching.Order{ID: 4, OwnerID: 1, Side: matching.Buy, Type: matching.Limit, Price: 5000, Qty: 5})
	if err != nil {
		t.Fatalf("BTC/USD 下单报错: %v", err)
	}
	if len(trades2) != 1 || trades2[0].Qty != 5 {
		t.Errorf("BTC/USD 应成交 5, got %+v", trades2)
	}

	// 验证余额
	// 用户 1: 100000 - 1000(ETH) - 25000(BTC) = 74000 USD, 10 ETH, 5 BTC
	if got := l.Available(1, "USD"); got != 74000 {
		t.Errorf("用户1 USD = %d, 期望 74000", got)
	}
	if got := l.Available(1, "ETH"); got != 10 {
		t.Errorf("用户1 ETH = %d, 期望 10", got)
	}
	if got := l.Available(1, "BTC"); got != 5 {
		t.Errorf("用户1 BTC = %d, 期望 5", got)
	}
}

// TestRouterUnknownSymbol 检查未注册币对返回错误。
func TestRouterUnknownSymbol(t *testing.T) {
	l := ledger.NewLedger()
	r := NewRouter(l)
	ctx := context.Background()

	_, err := r.Place(ctx, matching.Symbol{Base: "DOGE", Quote: "USD"}, matching.Order{ID: 1})
	if !errors.Is(err, ErrUnknownSymbol) {
		t.Errorf("未注册币对应返回 ErrUnknownSymbol, got %v", err)
	}

	err = r.Cancel(ctx, matching.Symbol{Base: "DOGE", Quote: "USD"}, 1)
	if !errors.Is(err, ErrUnknownSymbol) {
		t.Errorf("未注册币对撤单应返回 ErrUnknownSymbol, got %v", err)
	}
}

// TestEngineEmitsEvents 检查 Engine 成交后通过 EventBus 发出事件。
func TestEngineEmitsEvents(t *testing.T) {
	bus := eventbus.NewEventBus()
	e := NewEngine()
	e.SetEventBus(bus)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	_, ch := bus.Subscribe(10)

	// 挂一笔卖单(不成交, 只发 book_update)
	e.Place(ctx, matching.Order{ID: 1, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10})

	// 提交买单, 成交 → 应发 trade 事件 + book_update 事件
	e.Place(ctx, matching.Order{ID: 2, Side: matching.Buy, Type: matching.Limit, Price: 100, Qty: 10})

	// 收集事件
	var tradeEvents, bookEvents int
	timeout := time.After(time.Second)
	for {
		select {
		case ev := <-ch:
			switch ev.Type {
			case "trade":
				tradeEvents++
			case "book_update":
				bookEvents++
			}
		case <-timeout:
			goto done
		}
	}
done:
	if tradeEvents < 1 {
		t.Errorf("trade 事件 = %d, 期望 >= 1", tradeEvents)
	}
	if bookEvents < 1 {
		t.Errorf("book_update 事件 = %d, 期望 >= 1", bookEvents)
	}
}

// TestSubmitWithLedger 检查资金账户与撮合引擎的完整集成:
//
//	余额不足 → 拒单
//	正常成交 → 双方余额正确
//	撤单 → 解冻
func TestSubmitWithLedger(t *testing.T) {
	l := ledger.NewLedger()
	e := NewEngineWithLedger(l, "ETH", "USD")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	// --- 准备: 给两个用户入金 ---
	l.Deposit(1, "USD", 10000) // 用户 1: 10000 USD (买方)
	l.Deposit(2, "ETH", 100)   // 用户 2: 100 ETH (卖方)

	// --- 测试 1: 余额不足被拒 ---
	_, err := e.Place(ctx, matching.Order{
		ID: 99, OwnerID: 1, Side: matching.Buy, Type: matching.Limit, Price: 200, Qty: 100,
		// 需要 200*100=20000 USD, 但只有 10000
	})
	if !errors.Is(err, ledger.ErrInsufficientBalance) {
		t.Fatalf("余额不足应拒单, got err=%v", err)
	}
	// 被拒后余额不变
	if got := l.Available(1, "USD"); got != 10000 {
		t.Errorf("拒单后 USD 应不变, got %d", got)
	}

	// --- 测试 2: 正常成交 ---
	// 用户 2 挂卖单: 10 ETH @ 100
	_, err = e.Place(ctx, matching.Order{
		ID: 1, OwnerID: 2, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10,
	})
	if err != nil {
		t.Fatalf("挂卖单报错: %v", err)
	}
	// 卖方 ETH: 可用 90, 冻结 10
	if got := l.Available(2, "ETH"); got != 90 {
		t.Errorf("卖方可用 ETH = %d, 期望 90", got)
	}
	if got := l.Frozen(2, "ETH"); got != 10 {
		t.Errorf("卖方冻结 ETH = %d, 期望 10", got)
	}

	// 用户 1 提交买单: 10 ETH @ 100, 应成交
	trades, err := e.Place(ctx, matching.Order{
		ID: 2, OwnerID: 1, Side: matching.Buy, Type: matching.Limit, Price: 100, Qty: 10,
	})
	if err != nil {
		t.Fatalf("提交买单报错: %v", err)
	}
	if len(trades) != 1 || trades[0].Qty != 10 {
		t.Fatalf("应成交 10, got trades=%+v", trades)
	}

	// 成交后:
	// 买方: USD 10000-1000=9000, ETH +10
	if got := l.Available(1, "USD"); got != 9000 {
		t.Errorf("买方可用 USD = %d, 期望 9000", got)
	}
	if got := l.Available(1, "ETH"); got != 10 {
		t.Errorf("买方可用 ETH = %d, 期望 10", got)
	}
	// 卖方: ETH 冻结清零, USD +1000
	if got := l.Frozen(2, "ETH"); got != 0 {
		t.Errorf("卖方冻结 ETH = %d, 期望 0", got)
	}
	if got := l.Available(2, "USD"); got != 1000 {
		t.Errorf("卖方可用 USD = %d, 期望 1000", got)
	}

	// --- 测试 3: 撤单解冻 ---
	// 用户 1 挂一笔买单, 然后撤掉
	_, err = e.Place(ctx, matching.Order{
		ID: 3, OwnerID: 1, Side: matching.Buy, Type: matching.Limit, Price: 50, Qty: 10,
	})
	if err != nil {
		t.Fatalf("挂买单报错: %v", err)
	}
	// 冻结了 50*10=500 USD
	if got := l.Available(1, "USD"); got != 8500 {
		t.Errorf("挂买单后可用 USD = %d, 期望 8500", got)
	}

	err = e.Cancel(ctx, 3)
	if err != nil {
		t.Fatalf("撤单报错: %v", err)
	}
	// 解冻 500 → 可用恢复到 9000
	if got := l.Available(1, "USD"); got != 9000 {
		t.Errorf("撤单后可用 USD = %d, 期望 9000", got)
	}
}

// TestTradeStoreIntegration 检查成交后自动存入 TradeStore。
func TestTradeStoreIntegration(t *testing.T) {
	ts := store.NewMemoryStore()
	l := ledger.NewLedger()
	e := NewEngineWithLedger(l, "ETH", "USD")
	e.SetTradeStore(ts)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	// 入金
	l.Deposit(1, "USD", 100000)
	l.Deposit(2, "ETH", 100)

	// 挂卖单 + 买单成交
	e.Place(ctx, matching.Order{ID: 1, OwnerID: 2, Side: matching.Sell, Type: matching.Limit, Price: 100, Qty: 10})
	e.Place(ctx, matching.Order{ID: 2, OwnerID: 1, Side: matching.Buy, Type: matching.Limit, Price: 100, Qty: 10})

	// 等一下让 Engine 处理完
	time.Sleep(50 * time.Millisecond)

	// 查询成交记录
	trades, err := ts.ListTrades("ETH/USD", 10)
	if err != nil {
		t.Fatalf("ListTrades 报错: %v", err)
	}
	if len(trades) != 1 {
		t.Fatalf("trades = %d, 期望 1", len(trades))
	}
	if trades[0].Price != 100 || trades[0].Qty != 10 {
		t.Errorf("trade = {Price:%d, Qty:%d}, 期望 {100, 10}", trades[0].Price, trades[0].Qty)
	}
}
