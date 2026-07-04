package matching

import (
	"errors"
	"testing"
)

// TestNewBookWritable 检查 NewBook() 造出来的订单簿三个 map 都已 make(),可以直接写。
//
// 关键的 Java->Go 陷阱:Go 的零值 map 是 nil,读不 panic 但"写就 panic"。
// 下面被注释掉的一段演示了这个崩溃 —— 你可以取消注释、跑一次亲眼看它 panic,
// 然后再注释回去。
func TestNewBookWritable(t *testing.T) {
	b := NewBook()

	// NewBook 之后, 通过 AddOrder 写不该 panic。
	b.AddOrder(Order{ID: 1, Side: Buy, Price: 100, Qty: 10})
	b.AddOrder(Order{ID: 2, Side: Sell, Price: 101, Qty: 5})

	if got := b.bids.Get(100); got == nil || got.Price != 100 {
		t.Errorf("bids[100] 应存在且 Price=100")
	}
	if got := b.asks.Get(101); got == nil || got.Price != 101 {
		t.Errorf("asks[101] 应存在且 Price=101")
	}
	if got := b.orders[1].Qty; got != 10 {
		t.Errorf("orders[1].Qty = %d, 期望 10", got)
	}
}

// TestAddPreservesArrival 检查 AddOrder 在同一价位上保持到达顺序:
//
//	在价格 100 上先加订单 A(ID=1), 再加订单 B(ID=2),
//	档位里的 Orders 应该是 [1, 2] —— 先到的排前面。
//
// 同时检查:
//   - orders map 里能查到两笔订单
//   - locate 索引里两笔订单都指向价格 100
//
// aha: 如果 PriceLevel 以值(而非指针)存在 map 里,
// append 改的是副本, 真正的档位不会变 —— 这是 Java→Go 最大冲击。
func TestAddPreservesArrival(t *testing.T) {
	b := NewBook()

	o1 := Order{ID: 1, Side: Buy, Price: 100, Qty: 5}
	o2 := Order{ID: 2, Side: Buy, Price: 100, Qty: 3}
	b.AddOrder(o1)
	b.AddOrder(o2)

	// 1) 买方 100 档应该存在, 且队列是 [1, 2]
	lvl := b.bids.Get(100)
	if lvl == nil {
		t.Fatal("bids[100] 不存在")
	}
	if len(lvl.Orders) != 2 {
		t.Fatalf("bids[100].Orders 长度 = %d, 期望 2", len(lvl.Orders))
	}
	if lvl.Orders[0] != 1 || lvl.Orders[1] != 2 {
		t.Errorf("bids[100].Orders = %v, 期望 [1, 2]", lvl.Orders)
	}

	// 2) orders map 里两笔都能查到
	if _, exists := b.orders[1]; !exists {
		t.Errorf("orders[1] 应该存在")
	}
	if _, exists := b.orders[2]; !exists {
		t.Errorf("orders[2] 应该存在")
	}

	// 3) locate 索引: 两个 ID 都指向价格 100
	if p := b.locate[1]; p != 100 {
		t.Errorf("locate[1] = %d, 期望 100", p)
	}
	if p := b.locate[2]; p != 100 {
		t.Errorf("locate[2] = %d, 期望 100", p)
	}

	// 4) 再加一笔卖单, 确认买卖两侧互不干扰
	o3 := Order{ID: 3, Side: Sell, Price: 105, Qty: 7}
	b.AddOrder(o3)

	if b.asks.Get(105) == nil {
		t.Error("asks[105] 应该存在")
	}
	if len(b.bids.Get(100).Orders) != 2 {
		t.Error("加卖单不应影响买方档位")
	}
}

// TestBestPrice 检查 BestBid 和 BestAsk 能从无序 map 里正确找到最优价:
//   - 空簿 → ok=false
//   - 买方: 多个价格中取最高(买家出价越高越优先)
//   - 卖方: 多个价格中取最低(卖家要价越低越优先)
//   - 跑 50 次断言稳定(map 遍历顺序随机, 结果不能飘)
//
// aha: Go map 的 range 顺序是随机的, 不像 Java TreeMap 保证有序。
// 你必须显式遍历所有 key 求 max/min。
func TestBestPrice(t *testing.T) {
	b := NewBook()

	// 1) 空簿: 两边都该 ok=false
	if _, ok := b.BestBid(); ok {
		t.Error("空簿 BestBid 应返回 ok=false")
	}
	if _, ok := b.BestAsk(); ok {
		t.Error("空簿 BestAsk 应返回 ok=false")
	}

	// 2) 加三笔买单: 价格 100, 102, 101
	b.AddOrder(Order{ID: 1, Side: Buy, Price: 100, Qty: 5})
	b.AddOrder(Order{ID: 2, Side: Buy, Price: 102, Qty: 5})
	b.AddOrder(Order{ID: 3, Side: Buy, Price: 101, Qty: 5})

	// BestBid 应该是 102(最高买价)
	if price, ok := b.BestBid(); !ok || price != 102 {
		t.Errorf("BestBid() = (%d, %v), 期望 (102, true)", price, ok)
	}

	// 3) 加两笔卖单: 价格 105, 103
	b.AddOrder(Order{ID: 4, Side: Sell, Price: 105, Qty: 5})
	b.AddOrder(Order{ID: 5, Side: Sell, Price: 103, Qty: 5})

	// BestAsk 应该是 103(最低卖价)
	if price, ok := b.BestAsk(); !ok || price != 103 {
		t.Errorf("BestAsk() = (%d, %v), 期望 (103, true)", price, ok)
	}

	// 4) 稳定性: 跑 50 次, map 遍历顺序随机不应影响结果
	for i := 0; i < 50; i++ {
		if price, _ := b.BestBid(); price != 102 {
			t.Fatalf("第 %d 次 BestBid() = %d, 不稳定!", i, price)
		}
		if price, _ := b.BestAsk(); price != 103 {
			t.Fatalf("第 %d 次 BestAsk() = %d, 不稳定!", i, price)
		}
	}
}

// TestSubmitFullFill 检查最简单的撮合场景: 一笔 taker 恰好吃掉一笔 maker, 全部成交。
//
//	挂 Sell(price=100, qty=10) 作为 maker
//	Submit Buy(price=100, qty=10) 作为 taker
//	→ 恰好产出一笔 Trade{Price:100, Qty:10}
//	→ 成交后簿应该是空的
//
// aha: 成交价是 maker 的挂单价(100), 不是 taker 的限价。
func TestSubmitFullFill(t *testing.T) {
	b := NewBook()

	// 先挂一笔卖单(maker)
	b.AddOrder(Order{ID: 1, Side: Sell, Price: 100, Qty: 10})

	// 提交一笔买单(taker), 价格 >= 卖价, 数量刚好
	trades, _ := b.Submit(Order{ID: 2, Side: Buy, Price: 100, Qty: 10})

	// 应恰好一笔成交
	if len(trades) != 1 {
		t.Fatalf("trades 数量 = %d, 期望 1", len(trades))
	}

	tr := trades[0]

	// 成交价是 maker 的挂单价
	if tr.Price != 100 {
		t.Errorf("Trade.Price = %d, 期望 100(maker 价)", tr.Price)
	}
	if tr.Qty != 10 {
		t.Errorf("Trade.Qty = %d, 期望 10", tr.Qty)
	}
	if tr.TakerOrderID != 2 {
		t.Errorf("Trade.TakerOrderID = %d, 期望 2", tr.TakerOrderID)
	}
	if tr.MakerOrderID != 1 {
		t.Errorf("Trade.MakerOrderID = %d, 期望 1", tr.MakerOrderID)
	}

	// 成交后簿应该是空的: 卖方没有挂单了
	if _, ok := b.BestAsk(); ok {
		t.Error("全成交后 BestAsk 应返回 ok=false(簿空)")
	}
	// 买方也没挂上(因为全成交了, 没有剩余)
	if _, ok := b.BestBid(); ok {
		t.Error("全成交后 BestBid 应返回 ok=false(没有剩余挂上)")
	}
}

// TestSubmitPartialTakerBigger 检查 taker 数量 > maker 数量时的部分成交:
//
//	挂 Sell(100, 10), Submit Buy(100, 15)
//	→ Trade{Price:100, Qty:10}(只成交 maker 的全部 10)
//	→ taker 剩余 5 应挂到 bids[100]
func TestSubmitPartialTakerBigger(t *testing.T) {
	b := NewBook()

	b.AddOrder(Order{ID: 1, Side: Sell, Price: 100, Qty: 10})
	trades, _ := b.Submit(Order{ID: 2, Side: Buy, Price: 100, Qty: 15})

	// 应恰一笔成交, 数量是 min(15,10) = 10
	if len(trades) != 1 {
		t.Fatalf("trades 数量 = %d, 期望 1", len(trades))
	}
	if trades[0].Qty != 10 {
		t.Errorf("Trade.Qty = %d, 期望 10", trades[0].Qty)
	}
	if trades[0].Price != 100 {
		t.Errorf("Trade.Price = %d, 期望 100", trades[0].Price)
	}

	// maker 全成交, 卖方应该空了
	if _, ok := b.BestAsk(); ok {
		t.Error("maker 全成交后 BestAsk 应返回 ok=false")
	}

	// taker 剩余 5 应挂到买方
	bestBid, ok := b.BestBid()
	if !ok || bestBid != 100 {
		t.Fatalf("taker 剩余应挂到 bids, BestBid = (%d, %v), 期望 (100, true)", bestBid, ok)
	}
	if b.orders[2].Qty != 5 {
		t.Errorf("挂上后 orders[2].Qty = %d, 期望 5(剩余量)", b.orders[2].Qty)
	}
}

// TestSubmitPartialMakerBigger 检查 maker 数量 > taker 数量时的部分成交:
//
//	挂 Sell(100, 10), Submit Buy(100, 4)
//	→ Trade{Price:100, Qty:4}
//	→ maker 剩余 6, 仍挂在 asks[100]
//	→ taker 全成交, 不挂上
func TestSubmitPartialMakerBigger(t *testing.T) {
	b := NewBook()

	b.AddOrder(Order{ID: 1, Side: Sell, Price: 100, Qty: 10})
	trades, _ := b.Submit(Order{ID: 2, Side: Buy, Price: 100, Qty: 4})

	// 应恰一笔成交, 数量是 min(4,10) = 4
	if len(trades) != 1 {
		t.Fatalf("trades 数量 = %d, 期望 1", len(trades))
	}
	if trades[0].Qty != 4 {
		t.Errorf("Trade.Qty = %d, 期望 4", trades[0].Qty)
	}

	// maker 还剩 6, 仍挂在卖方
	bestAsk, ok := b.BestAsk()
	if !ok || bestAsk != 100 {
		t.Fatalf("maker 应还在, BestAsk = (%d, %v), 期望 (100, true)", bestAsk, ok)
	}
	if b.orders[1].Qty != 6 {
		t.Errorf("maker 剩余 orders[1].Qty = %d, 期望 6", b.orders[1].Qty)
	}

	// taker 全成交, 不应挂到买方
	if _, ok := b.BestBid(); ok {
		t.Error("taker 全成交, 不应挂到买方")
	}
}

// TestSubmitMultiLevel 检查跨档扫单(价格优先):
//
//	挂 Sell(100,5) 和 Sell(101,5)
//	Submit Buy(限价101, qty=8)
//	→ 先吃最优卖价 100, Trade(100,5)
//	→ 再吃次优卖价 101, Trade(101,3)
//	→ 卖 101 剩 2
//
// aha: 每轮重新取 BestAsk, 保证价格优先; 循环直到"无量或不交叉"。
func TestSubmitMultiLevel(t *testing.T) {
	b := NewBook()

	b.AddOrder(Order{ID: 1, Side: Sell, Price: 100, Qty: 5})
	b.AddOrder(Order{ID: 2, Side: Sell, Price: 101, Qty: 5})

	trades, _ := b.Submit(Order{ID: 3, Side: Buy, Price: 101, Qty: 8})

	// 应产出两笔成交
	if len(trades) != 2 {
		t.Fatalf("trades 数量 = %d, 期望 2", len(trades))
	}

	// 第一笔: 先吃最优价 100, 全部 5
	if trades[0].Price != 100 || trades[0].Qty != 5 {
		t.Errorf("trades[0] = {Price:%d, Qty:%d}, 期望 {100, 5}", trades[0].Price, trades[0].Qty)
	}
	// 第二笔: 再吃 101, 只需 3
	if trades[1].Price != 101 || trades[1].Qty != 3 {
		t.Errorf("trades[1] = {Price:%d, Qty:%d}, 期望 {101, 3}", trades[1].Price, trades[1].Qty)
	}

	// 卖 101 应剩 2
	if b.orders[2].Qty != 2 {
		t.Errorf("orders[2].Qty = %d, 期望 2", b.orders[2].Qty)
	}

	// taker 全成交, 不该挂上
	if _, ok := b.BestBid(); ok {
		t.Error("taker 全成交, 不应挂到买方")
	}
}

// TestSubmitSellTaker 检查 Sell taker 吃 Buy maker 的对称逻辑:
//
//	挂 Buy(102,5) 和 Buy(100,5)
//	Submit Sell(限价100, qty=8)
//	→ 先吃最优买价 102, Trade(102,5)
//	→ 再吃次优买价 100, Trade(100,3)
func TestSubmitSellTaker(t *testing.T) {
	b := NewBook()

	b.AddOrder(Order{ID: 1, Side: Buy, Price: 102, Qty: 5})
	b.AddOrder(Order{ID: 2, Side: Buy, Price: 100, Qty: 5})

	trades, _ := b.Submit(Order{ID: 3, Side: Sell, Price: 100, Qty: 8})

	if len(trades) != 2 {
		t.Fatalf("trades 数量 = %d, 期望 2", len(trades))
	}

	// 先吃最优买价 102
	if trades[0].Price != 102 || trades[0].Qty != 5 {
		t.Errorf("trades[0] = {Price:%d, Qty:%d}, 期望 {102, 5}", trades[0].Price, trades[0].Qty)
	}
	// 再吃 100, 只需 3
	if trades[1].Price != 100 || trades[1].Qty != 3 {
		t.Errorf("trades[1] = {Price:%d, Qty:%d}, 期望 {100, 3}", trades[1].Price, trades[1].Qty)
	}

	// 买 100 应剩 2
	if b.orders[2].Qty != 2 {
		t.Errorf("orders[2].Qty = %d, 期望 2", b.orders[2].Qty)
	}

	// taker 全成交, 不该挂上
	if _, ok := b.BestAsk(); ok {
		t.Error("Sell taker 全成交, 不应挂到卖方")
	}
}

// TestSubmitFIFO 检查同一档位内的时间优先:
//
//	挂 SellA(100,5) 再挂 SellB(100,5), A 先到
//	Submit Buy(100,5)
//	→ 只跟 A 成交, B 不动
//
// aha: 取的是 lvl.Orders[0](队首), 时间优先从 FIFO 队列免费得到。
func TestSubmitFIFO(t *testing.T) {
	b := NewBook()

	// A 先到, B 后到, 同价 100
	b.AddOrder(Order{ID: 1, Side: Sell, Price: 100, Qty: 5})
	b.AddOrder(Order{ID: 2, Side: Sell, Price: 100, Qty: 5})

	// taker 只买 5, 应该只吃 A
	trades, _ := b.Submit(Order{ID: 3, Side: Buy, Price: 100, Qty: 5})

	if len(trades) != 1 {
		t.Fatalf("trades 数量 = %d, 期望 1", len(trades))
	}

	// 成交的是 A(ID=1), 不是 B(ID=2)
	if trades[0].MakerOrderID != 1 {
		t.Errorf("MakerOrderID = %d, 期望 1(A 先到应先成交)", trades[0].MakerOrderID)
	}

	// B(ID=2) 应该还在, 数量不变
	if b.orders[2].Qty != 5 {
		t.Errorf("B 不应被动, orders[2].Qty = %d, 期望 5", b.orders[2].Qty)
	}

	// asks[100] 应该只剩 B
	lvl := b.asks.Get(100)
	if lvl == nil || len(lvl.Orders) != 1 || lvl.Orders[0] != 2 {
		t.Errorf("asks[100] 应只剩 [2], 实际 %v", lvl.Orders)
	}
}

// TestSubmitNoCross 检查不交叉时零成交, taker 直接挂上:
//
//	场景 1: 最优卖 100, Buy(限价99,5) → 不交叉, 零 Trade, 买单挂到 bids[99]
//	场景 2: 空簿, Buy(限价99,5) → 无对手, 零 Trade, 买单挂到 bids[99]
//
// aha: 挂单 = "撮合循环没产出任何东西, 于是 AddOrder"。
func TestSubmitNoCross(t *testing.T) {
	// 场景 1: 有对手但不交叉
	b := NewBook()
	b.AddOrder(Order{ID: 1, Side: Sell, Price: 100, Qty: 10})

	trades, _ := b.Submit(Order{ID: 2, Side: Buy, Price: 99, Qty: 5})

	if len(trades) != 0 {
		t.Errorf("不交叉应零成交, trades 数量 = %d", len(trades))
	}

	// 买单应挂到 bids[99]
	bestBid, ok := b.BestBid()
	if !ok || bestBid != 99 {
		t.Errorf("BestBid = (%d, %v), 期望 (99, true)", bestBid, ok)
	}
	if b.orders[2].Qty != 5 {
		t.Errorf("挂上后 orders[2].Qty = %d, 期望 5", b.orders[2].Qty)
	}

	// 卖方不受影响
	if b.orders[1].Qty != 10 {
		t.Errorf("卖方不应被动, orders[1].Qty = %d, 期望 10", b.orders[1].Qty)
	}
}

func TestSubmitEmptyBook(t *testing.T) {
	// 场景 2: 空簿直接挂上
	b := NewBook()

	trades, _ := b.Submit(Order{ID: 1, Side: Buy, Price: 99, Qty: 5})

	if len(trades) != 0 {
		t.Errorf("空簿应零成交, trades 数量 = %d", len(trades))
	}

	bestBid, ok := b.BestBid()
	if !ok || bestBid != 99 {
		t.Errorf("BestBid = (%d, %v), 期望 (99, true)", bestBid, ok)
	}
}

// TestSubmitMarketEmptyBook 检查市价单进空簿:
//
//	空簿, Submit Market Buy(qty=5)
//	→ 零 Trade, 剩余 5 不挂上
//
// aha: 市价单永不挂上, "无流动性"响亮地暴露为未成交。
func TestSubmitMarketEmptyBook(t *testing.T) {
	b := NewBook()

	trades, _ := b.Submit(Order{ID: 1, Side: Buy, Type: Market, Qty: 5})

	if len(trades) != 0 {
		t.Errorf("空簿市价单应零成交, trades 数量 = %d", len(trades))
	}

	// 市价单不挂上
	if _, ok := b.BestBid(); ok {
		t.Error("市价单剩余不应挂到簿上")
	}
	if _, exists := b.orders[1]; exists {
		t.Error("市价单不应出现在 orders 中")
	}
}

// TestSubmitMarketPartial 检查市价单部分成交后剩余不挂:
//
//	挂 Sell(100, 3), Submit Market Buy(qty=10)
//	→ Trade(100, 3), 剩余 7 不挂上
func TestSubmitMarketPartial(t *testing.T) {
	b := NewBook()

	b.AddOrder(Order{ID: 1, Side: Sell, Price: 100, Qty: 3})
	trades, _ := b.Submit(Order{ID: 2, Side: Buy, Type: Market, Qty: 10})

	// 应成交 3
	if len(trades) != 1 {
		t.Fatalf("trades 数量 = %d, 期望 1", len(trades))
	}
	if trades[0].Qty != 3 {
		t.Errorf("Trade.Qty = %d, 期望 3", trades[0].Qty)
	}
	if trades[0].Price != 100 {
		t.Errorf("Trade.Price = %d, 期望 100", trades[0].Price)
	}

	// 剩余 7 不挂上
	if _, ok := b.BestBid(); ok {
		t.Error("市价单剩余 7 不应挂到买方")
	}

	// 未成交量 = 10 - 3 = 7
	var filled int64
	for _, tr := range trades {
		filled += tr.Qty
	}
	if remaining := int64(10) - filled; remaining != 7 {
		t.Errorf("未成交量 = %d, 期望 7", remaining)
	}
}

// TestSubmitValidation 检查 Submit 的输入校验:
//   - Qty <= 0 → 拒绝
//   - 限价单 Price <= 0 → 拒绝
//   - 被拒绝后簿不变
func TestSubmitValidation(t *testing.T) {
	b := NewBook()

	// Qty = 0
	_, err := b.Submit(Order{ID: 1, Side: Buy, Type: Limit, Price: 100, Qty: 0})
	if !errors.Is(err, ErrInvalidQty) {
		t.Errorf("Qty=0 应返回 ErrInvalidQty, got %v", err)
	}

	// Qty < 0
	_, err = b.Submit(Order{ID: 2, Side: Buy, Type: Limit, Price: 100, Qty: -5})
	if !errors.Is(err, ErrInvalidQty) {
		t.Errorf("Qty<0 应返回 ErrInvalidQty, got %v", err)
	}

	// 限价单 Price = 0
	_, err = b.Submit(Order{ID: 3, Side: Buy, Type: Limit, Price: 0, Qty: 10})
	if !errors.Is(err, ErrInvalidPrice) {
		t.Errorf("限价 Price=0 应返回 ErrInvalidPrice, got %v", err)
	}

	// 簿应该是空的(被拒绝的单不应进入簿)
	if _, ok := b.BestBid(); ok {
		t.Error("被拒绝的单不应出现在簿中")
	}
}

// TestSubmitSelfTradePrevention 检查自成交防护(策略: 撤新单):
//
//	用户 X(OwnerID=100) 挂 Sell(100, 5)
//	用户 X 又 Submit Buy(100, 5)
//	→ 检测到自成交, 拒绝新单, 零 Trade, 原挂单不动
func TestSubmitSelfTradePrevention(t *testing.T) {
	b := NewBook()

	// 用户 X 挂一笔卖单
	b.AddOrder(Order{ID: 1, OwnerID: 100, Side: Sell, Price: 100, Qty: 5})

	// 用户 X 又提交买单, 会跟自己成交 → 应被拒绝
	trades, err := b.Submit(Order{ID: 2, OwnerID: 100, Side: Buy, Type: Limit, Price: 100, Qty: 5})

	if err != nil {
		t.Errorf("自成交防护不应返回 error, got %v", err)
	}
	if len(trades) != 0 {
		t.Errorf("自成交应零 Trade, got %d", len(trades))
	}

	// 原挂单不应被动
	if b.orders[1].Qty != 5 {
		t.Errorf("原挂单 Qty = %d, 期望 5(不应被动)", b.orders[1].Qty)
	}

	// 新单不应挂上
	if _, exists := b.orders[2]; exists {
		t.Error("被拒的新单不应出现在 orders 中")
	}
}

// TestSubmitDifferentOwnerCanTrade 确认不同用户可以正常成交。
func TestSubmitDifferentOwnerCanTrade(t *testing.T) {
	b := NewBook()

	b.AddOrder(Order{ID: 1, OwnerID: 100, Side: Sell, Price: 100, Qty: 5})
	trades, _ := b.Submit(Order{ID: 2, OwnerID: 200, Side: Buy, Type: Limit, Price: 100, Qty: 5})

	if len(trades) != 1 {
		t.Fatalf("不同用户应正常成交, trades = %d", len(trades))
	}
}

// TestSubmitDeterministic 检查确定性:
// 同样的订单序列跑两遍, 产出的 Trade 应该逐字段相同。
func TestSubmitDeterministic(t *testing.T) {
	run := func() []Trade {
		b := NewBook()
		b.AddOrder(Order{ID: 1, Side: Sell, Price: 100, Qty: 5})
		b.AddOrder(Order{ID: 2, Side: Sell, Price: 101, Qty: 5})
		b.AddOrder(Order{ID: 3, Side: Sell, Price: 102, Qty: 5})
		trades, _ := b.Submit(Order{ID: 4, Side: Buy, Type: Limit, Price: 101, Qty: 8})
		return trades
	}

	t1 := run()
	// 跑 20 次, 每次都应该跟第一次完全一样
	for i := 0; i < 20; i++ {
		t2 := run()
		if len(t1) != len(t2) {
			t.Fatalf("第 %d 次: trades 数量 %d != %d", i, len(t2), len(t1))
		}
		for j := range t1 {
			if t1[j] != t2[j] {
				t.Fatalf("第 %d 次: trades[%d] = %+v, 期望 %+v", i, j, t2[j], t1[j])
			}
		}
	}
}

// TestCancel 检查撤单:
//   - 档位 [A, B, C], 撤中间 B → 剩 [A, C], 顺序不变
//   - 全部撤完 → 档位从 map 中删除, BestBid 看不到该价格
//
// aha: 忘删空档会留"幽灵价", BestBid 会返回一个已经没有订单的价格。
func TestCancel(t *testing.T) {
	b := NewBook()

	// 在价格 100 上挂三笔买单: A=1, B=2, C=3
	b.AddOrder(Order{ID: 1, Side: Buy, Price: 100, Qty: 5})
	b.AddOrder(Order{ID: 2, Side: Buy, Price: 100, Qty: 5})
	b.AddOrder(Order{ID: 3, Side: Buy, Price: 100, Qty: 5})

	// 撤中间 B(ID=2) → 档位应剩 [1, 3], 顺序不变
	if err := b.CancelOrder(2); err != nil {
		t.Fatalf("CancelOrder(2) 报错: %v", err)
	}

	lvl := b.bids.Get(100)
	if lvl == nil {
		t.Fatal("撤 B 后 bids[100] 不应该消失")
	}
	if len(lvl.Orders) != 2 {
		t.Fatalf("撤 B 后 Orders 长度 = %d, 期望 2", len(lvl.Orders))
	}
	if lvl.Orders[0] != 1 || lvl.Orders[1] != 3 {
		t.Errorf("撤 B 后 Orders = %v, 期望 [1, 3]", lvl.Orders)
	}

	// orders 和 locate 里不再有 ID=2
	if _, exists := b.orders[2]; exists {
		t.Error("orders[2] 应该已删除")
	}
	if _, exists := b.locate[2]; exists {
		t.Error("locate[2] 应该已删除")
	}

	// 撤 A(ID=1), 再撤 C(ID=3) → 档位变空, 应从 bids map 中删除
	if err := b.CancelOrder(1); err != nil {
		t.Fatalf("CancelOrder(1) 报错: %v", err)
	}
	if err := b.CancelOrder(3); err != nil {
		t.Fatalf("CancelOrder(3) 报错: %v", err)
	}

	// 档位应该被清理掉
	if b.bids.Get(100) != nil {
		t.Error("全部撤完后 bids[100] 应该被删除(避免幽灵价)")
	}

	// BestBid 不该再看到价格 100
	if _, ok := b.BestBid(); ok {
		t.Error("全部撤完后 BestBid 应返回 ok=false")
	}
}

// TestCancelUnknown 检查撤一个不存在的订单, 应返回 ErrNotFound。
func TestCancelUnknown(t *testing.T) {
	b := NewBook()

	err := b.CancelOrder(999)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("CancelOrder(999) = %v, 期望 ErrNotFound", err)
	}
}

// TestModify 检查 ModifyOrder 的价格时间优先语义:
//
//	纯减量 → 保持队列位置
//	增量   → 排到队尾
//	改价   → 移到新档队尾
//
// aha: 交易所公平规则 — 只有减量能保住队位, 否则就得重新排队。
func TestModify(t *testing.T) {
	b := NewBook()

	// 在价格 100 上挂三笔买单: A=1, B=2, C=3, 队列 [1, 2, 3]
	b.AddOrder(Order{ID: 1, Side: Buy, Price: 100, Qty: 10})
	b.AddOrder(Order{ID: 2, Side: Buy, Price: 100, Qty: 10})
	b.AddOrder(Order{ID: 3, Side: Buy, Price: 100, Qty: 10})

	// --- 纯减量: B(ID=2) 从 10 减到 5, 价格不变 ---
	// 应保持队中位置 [1, 2, 3], Qty 变 5
	if err := b.ModifyOrder(2, 100, 5); err != nil {
		t.Fatalf("纯减量报错: %v", err)
	}
	lvl := b.bids.Get(100)
	if lvl.Orders[0] != 1 || lvl.Orders[1] != 2 || lvl.Orders[2] != 3 {
		t.Errorf("纯减量后 Orders = %v, 期望 [1, 2, 3](位置不变)", lvl.Orders)
	}
	if b.orders[2].Qty != 5 {
		t.Errorf("纯减量后 orders[2].Qty = %d, 期望 5", b.orders[2].Qty)
	}

	// --- 增量: B(ID=2) 从 5 增到 20, 价格不变 ---
	// 应排到队尾 [1, 3, 2]
	if err := b.ModifyOrder(2, 100, 20); err != nil {
		t.Fatalf("增量报错: %v", err)
	}
	lvl = b.bids.Get(100)
	if len(lvl.Orders) != 3 {
		t.Fatalf("增量后 Orders 长度 = %d, 期望 3", len(lvl.Orders))
	}
	if lvl.Orders[0] != 1 || lvl.Orders[1] != 3 || lvl.Orders[2] != 2 {
		t.Errorf("增量后 Orders = %v, 期望 [1, 3, 2](B 排到队尾)", lvl.Orders)
	}
	if b.orders[2].Qty != 20 {
		t.Errorf("增量后 orders[2].Qty = %d, 期望 20", b.orders[2].Qty)
	}

	// --- 改价: B(ID=2) 从 100 改到 105 ---
	// 应从 bids[100] 消失, 出现在 bids[105] 队尾
	if err := b.ModifyOrder(2, 105, 20); err != nil {
		t.Fatalf("改价报错: %v", err)
	}
	lvl100 := b.bids.Get(100)
	if len(lvl100.Orders) != 2 {
		t.Fatalf("改价后 bids[100] 长度 = %d, 期望 2", len(lvl100.Orders))
	}
	if lvl100.Orders[0] != 1 || lvl100.Orders[1] != 3 {
		t.Errorf("改价后 bids[100] = %v, 期望 [1, 3]", lvl100.Orders)
	}
	lvl105 := b.bids.Get(105)
	if lvl105 == nil || len(lvl105.Orders) != 1 || lvl105.Orders[0] != 2 {
		t.Errorf("改价后 bids[105] 应只有 [2]")
	}
	if b.locate[2] != 105 {
		t.Errorf("改价后 locate[2] = %d, 期望 105", b.locate[2])
	}
}

// TestModifyNotFound 检查修改不存在的订单返回 ErrNotFound。
func TestModifyNotFound(t *testing.T) {
	b := NewBook()
	if err := b.ModifyOrder(999, 100, 5); !errors.Is(err, ErrNotFound) {
		t.Errorf("ModifyOrder(999) = %v, 期望 ErrNotFound", err)
	}
}

// TestModifyInvalidQty 检查新数量 <= 0 时返回 ErrInvalidQty。
func TestModifyInvalidQty(t *testing.T) {
	b := NewBook()
	b.AddOrder(Order{ID: 1, Side: Buy, Price: 100, Qty: 10})

	if err := b.ModifyOrder(1, 100, 0); !errors.Is(err, ErrInvalidQty) {
		t.Errorf("ModifyOrder(qty=0) = %v, 期望 ErrInvalidQty", err)
	}
	if err := b.ModifyOrder(1, 100, -5); !errors.Is(err, ErrInvalidQty) {
		t.Errorf("ModifyOrder(qty=-5) = %v, 期望 ErrInvalidQty", err)
	}
}
