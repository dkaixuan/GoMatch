package matching

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

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
		Trades []Trade `json:"trades"`
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

	var snap BookSnapshot
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
	bus := NewEventBus()
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
	e.Place(ctx, Order{ID: 1, Side: Sell, Type: Limit, Price: 100, Qty: 10})
	e.Place(ctx, Order{ID: 2, Side: Buy, Type: Limit, Price: 100, Qty: 10})

	// 应收到事件
	ws.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := ws.ReadMessage()
	if err != nil {
		t.Fatalf("读 WebSocket 消息失败: %v", err)
	}

	var event Event
	if err := json.Unmarshal(msg, &event); err != nil {
		t.Fatalf("解析事件 JSON 失败: %v", err)
	}

	if event.Type != "trade" && event.Type != "book_update" {
		t.Errorf("事件类型 = %q, 期望 trade 或 book_update", event.Type)
	}
}
