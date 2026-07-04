package mm

import (
	"context"
	"testing"
	"time"
)

// TestFakeImplementsExchange 编译期检查: fakeExchange 满足 Exchange 接口。
// 如果 fake 缺少任何方法, 这行会编译报错, 测试都跑不起来。
//
// aha: Go 接口是隐式的, 不用写 implements。
// 这行 "var _ Exchange = (*fakeExchange)(nil)" 是惯用的编译期断言。
var _ Exchange = (*fakeExchange)(nil)

func TestFakeImplementsExchange(t *testing.T) {
	f := newFakeExchange()

	// 下两笔单
	id1, err := f.PlaceLimit("buy", 100, 5)
	if err != nil || id1 != 1 {
		t.Errorf("PlaceLimit 1: id=%d, err=%v", id1, err)
	}
	id2, err := f.PlaceLimit("sell", 105, 3)
	if err != nil || id2 != 2 {
		t.Errorf("PlaceLimit 2: id=%d, err=%v", id2, err)
	}

	// 检查记录
	if len(f.placed) != 2 {
		t.Fatalf("placed 数量 = %d, 期望 2", len(f.placed))
	}
	if f.placed[0].Side != "buy" || f.placed[0].Price != 100 {
		t.Errorf("placed[0] = %+v, 期望 buy@100", f.placed[0])
	}

	// 撤单
	f.Cancel(1)
	if len(f.canceled) != 1 || f.canceled[0] != 1 {
		t.Errorf("canceled = %v, 期望 [1]", f.canceled)
	}

	// BestBid/BestAsk
	f.bestBid = 99
	f.hasBid = true
	if p, ok := f.BestBid(); !ok || p != 99 {
		t.Errorf("BestBid = (%d, %v), 期望 (99, true)", p, ok)
	}
}

// TestQuotesFixedSpread 检查固定价差报价(纯函数):
//
//	mid=10000, SpreadBps=200(2%) → bid=9900, ask=10100, 差 200
//	mid=10000, SpreadBps=20(0.2%) → bid=9990, ask=10010, 差 20
//	对称性: ask - mid == mid - bid
func TestQuotesFixedSpread(t *testing.T) {
	cases := []struct {
		name      string
		spreadBps int64
		mid       int64
		wantBid   int64
		wantAsk   int64
	}{
		{"2%价差", 200, 10000, 9900, 10100},
		{"0.2%价差", 20, 10000, 9990, 10010},
		{"1%价差", 100, 10000, 9950, 10050},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			mm := &MarketMaker{SpreadBps: c.spreadBps}
			bid, ask := mm.Quotes(c.mid)

			if bid != c.wantBid {
				t.Errorf("bid = %d, 期望 %d", bid, c.wantBid)
			}
			if ask != c.wantAsk {
				t.Errorf("ask = %d, 期望 %d", ask, c.wantAsk)
			}
			// 对称性
			if ask-c.mid != c.mid-bid {
				t.Errorf("不对称: ask-mid=%d, mid-bid=%d", ask-c.mid, c.mid-bid)
			}
		})
	}
}

// TestStepPlacesTwo 检查 Step 下恰好两笔单: 一买一卖。
func TestStepPlacesTwo(t *testing.T) {
	f := newFakeExchange()
	mm := &MarketMaker{
		Exchange:  f,
		SpreadBps: 200, // 2%
		Qty:       5,
	}

	mm.Step(10000)

	// 应恰好下了两笔单
	if len(f.placed) != 2 {
		t.Fatalf("placed 数量 = %d, 期望 2", len(f.placed))
	}

	// 一笔 buy@9900, 一笔 sell@10100
	buy := f.placed[0]
	sell := f.placed[1]
	if buy.Side != "buy" || buy.Price != 9900 || buy.Qty != 5 {
		t.Errorf("buy = %+v, 期望 {buy, 9900, 5}", buy)
	}
	if sell.Side != "sell" || sell.Price != 10100 || sell.Qty != 5 {
		t.Errorf("sell = %+v, 期望 {sell, 10100, 5}", sell)
	}

	// 第一次 Step 不该有撤单(没有旧单可撤)
	if len(f.canceled) != 0 {
		t.Errorf("第一次 Step 不该撤单, canceled = %v", f.canceled)
	}
}

// TestStepCancelsBeforeReplace 检查第二次 Step 先撤旧单再下新单。
func TestStepCancelsBeforeReplace(t *testing.T) {
	f := newFakeExchange()
	mm := &MarketMaker{
		Exchange:  f,
		SpreadBps: 200,
		Qty:       5,
	}

	// 第一次 Step
	mm.Step(10000)
	// fake 分配了 ID=1(buy) 和 ID=2(sell)

	// 第二次 Step, 价格变了
	mm.Step(10050)

	// 应先撤掉 ID=1 和 ID=2
	if len(f.canceled) != 2 {
		t.Fatalf("canceled 数量 = %d, 期望 2", len(f.canceled))
	}
	if f.canceled[0] != 1 || f.canceled[1] != 2 {
		t.Errorf("canceled = %v, 期望 [1, 2]", f.canceled)
	}

	// 总共下了 4 笔单(两次各两笔)
	if len(f.placed) != 4 {
		t.Fatalf("placed 数量 = %d, 期望 4", len(f.placed))
	}

	// 最后两笔是新的报价
	newBuy := f.placed[2]
	newSell := f.placed[3]
	// mid=10050, halfSpread=10050*200/2/10000=100 → bid=9950, ask=10150
	if newBuy.Side != "buy" || newBuy.Price != 9950 {
		t.Errorf("新 buy = %+v, 期望 buy@9950", newBuy)
	}
	if newSell.Side != "sell" || newSell.Price != 10150 {
		t.Errorf("新 sell = %+v, 期望 sell@10150", newSell)
	}
}

// TestOnFill 检查库存追踪:
//   Buy 成交 5 → inventory = +5
//   Sell 成交 3 → inventory = +2
//   Sell 成交 2 → inventory = 0
func TestOnFill(t *testing.T) {
	mm := &MarketMaker{}

	mm.OnFill("buy", 5)
	if mm.Inventory != 5 {
		t.Errorf("buy 5 后 inventory = %d, 期望 5", mm.Inventory)
	}

	mm.OnFill("sell", 3)
	if mm.Inventory != 2 {
		t.Errorf("sell 3 后 inventory = %d, 期望 2", mm.Inventory)
	}

	mm.OnFill("sell", 2)
	if mm.Inventory != 0 {
		t.Errorf("sell 2 后 inventory = %d, 期望 0", mm.Inventory)
	}
}

// TestSkew 检查库存偏移:
//   inventory=0 → 跟 Stage-1 对称报价一致
//   inventory>0 → center 左移(倾向卖), ask 比 bid 更靠近 mid
//   inventory<0 → center 右移(倾向买)
func TestSkew(t *testing.T) {
	// inventory=0: 应跟固定价差一样(向后兼容)
	t.Run("zero_inventory", func(t *testing.T) {
		mm := &MarketMaker{SpreadBps: 200, SkewBps: 100, MaxInventory: 10}
		mm.Inventory = 0
		bid, ask := mm.Quotes(10000)
		if bid != 9900 || ask != 10100 {
			t.Errorf("inventory=0: bid=%d, ask=%d, 期望 (9900, 10100)", bid, ask)
		}
	})

	// inventory>0 (持多): center 应左移
	t.Run("positive_inventory", func(t *testing.T) {
		mm := &MarketMaker{SpreadBps: 200, SkewBps: 100, MaxInventory: 10}
		mm.Inventory = 5 // 半仓

		bid, ask := mm.Quotes(10000)
		mid := int64(10000)

		// center 应 < mid
		center := (bid + ask) / 2
		if center >= mid {
			t.Errorf("持多时 center=%d 应 < mid=%d", center, mid)
		}
		// ask 比 bid 更靠近 mid(倾向卖)
		if (mid - bid) <= (ask - mid) {
			t.Errorf("持多时 ask 应比 bid 更靠近 mid: mid-bid=%d, ask-mid=%d", mid-bid, ask-mid)
		}
	})

	// inventory<0 (持空): center 应右移
	t.Run("negative_inventory", func(t *testing.T) {
		mm := &MarketMaker{SpreadBps: 200, SkewBps: 100, MaxInventory: 10}
		mm.Inventory = -5

		bid, ask := mm.Quotes(10000)
		mid := int64(10000)

		// center 应 > mid
		center := (bid + ask) / 2
		if center <= mid {
			t.Errorf("持空时 center=%d 应 > mid=%d", center, mid)
		}
	})
}

// TestMaxInventory 检查硬仓位上限:
//   inventory = +MaxInventory 时, Step 只挂卖单不挂买单
//   inventory = -MaxInventory 时, Step 只挂买单不挂卖单
func TestMaxInventory(t *testing.T) {
	t.Run("full_long", func(t *testing.T) {
		f := newFakeExchange()
		mm := &MarketMaker{
			Exchange: f, SpreadBps: 200, Qty: 5, MaxInventory: 10,
		}
		mm.Inventory = 10 // 满仓多头

		mm.Step(10000)

		// 应只下了一笔卖单, 没有买单
		if len(f.placed) != 1 {
			t.Fatalf("满仓多头 placed 数量 = %d, 期望 1(只卖)", len(f.placed))
		}
		if f.placed[0].Side != "sell" {
			t.Errorf("满仓多头应只挂卖单, got %s", f.placed[0].Side)
		}
	})

	t.Run("full_short", func(t *testing.T) {
		f := newFakeExchange()
		mm := &MarketMaker{
			Exchange: f, SpreadBps: 200, Qty: 5, MaxInventory: 10,
		}
		mm.Inventory = -10 // 满仓空头

		mm.Step(10000)

		// 应只下了一笔买单, 没有卖单
		if len(f.placed) != 1 {
			t.Fatalf("满仓空头 placed 数量 = %d, 期望 1(只买)", len(f.placed))
		}
		if f.placed[0].Side != "buy" {
			t.Errorf("满仓空头应只挂买单, got %s", f.placed[0].Side)
		}
	})
}

// TestSelfCross 检查自穿越保护:
//   手动构造 bid >= ask 的情况, Step 应不下任何单。
func TestSelfCross(t *testing.T) {
	f := newFakeExchange()
	mm := &MarketMaker{
		Exchange:     f,
		SpreadBps:    1,      // 极小价差: halfSpread ≈ 0
		SkewBps:      20000,  // 极大偏移
		Qty:          5,
		MaxInventory: 1,      // MaxInventory=1, 所以 inventory=1 就是满偏移
	}
	mm.Inventory = 1

	bid, ask := mm.Quotes(100)
	t.Logf("bid=%d, ask=%d", bid, ask)

	if bid < ask {
		t.Skipf("参数没产生交叉 bid=%d ask=%d, 跳过", bid, ask)
	}

	mm.Step(100)

	if len(f.placed) != 0 {
		t.Errorf("自穿越时不应下单, placed 数量 = %d", len(f.placed))
	}
}

// TestLoop 检查 Run 主循环:
//   喂合成 tick 和 fill, 断言 Step/OnFill 被正确调用, inventory 终态正确。
func TestLoop(t *testing.T) {
	f := newFakeExchange()
	mm := &MarketMaker{
		Exchange:     f,
		SpreadBps:    200,
		Qty:          5,
		MaxInventory: 100,
	}

	ctx, cancel := context.WithCancel(context.Background())
	ticker := make(chan struct{}, 10)
	fills := make(chan Fill, 10)

	done := make(chan struct{})
	go func() {
		mm.Run(ctx, func() int64 { return 10000 }, ticker, fills)
		close(done)
	}()

	// 触发两次 tick → 应各下两笔单
	ticker <- struct{}{}
	ticker <- struct{}{}

	// 喂两笔成交
	fills <- Fill{Side: "buy", Qty: 5}
	fills <- Fill{Side: "sell", Qty: 3}

	// 等一小会让 Run 处理完
	time.Sleep(50 * time.Millisecond)

	// 关停
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Run 应在 ctx 取消后退出, 但超时了")
	}

	// 检查 inventory: +5 - 3 = 2
	if mm.Inventory != 2 {
		t.Errorf("inventory = %d, 期望 2", mm.Inventory)
	}

	// 检查有下过单 (至少 2 次 tick × 2 笔 = 4 笔)
	if len(f.placed) < 4 {
		t.Errorf("placed 数量 = %d, 期望 >= 4", len(f.placed))
	}
}