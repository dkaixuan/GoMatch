package matching

import (
	"context"
	"errors"
	"testing"
)

// TestRouterTwoPairs 检查两个币对各自下单互不干扰。
func TestRouterTwoPairs(t *testing.T) {
	l := NewLedger()
	r := NewRouter(l)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	eth := Symbol{Base: "ETH", Quote: "USD"}
	btc := Symbol{Base: "BTC", Quote: "USD"}
	r.Register(ctx, eth)
	r.Register(ctx, btc)

	// 给用户入金
	l.Deposit(1, "USD", 100000)
	l.Deposit(2, "ETH", 100)
	l.Deposit(3, "BTC", 50)

	// ETH/USD: 用户 2 挂卖 10 ETH @ 100
	r.Place(ctx, eth, Order{ID: 1, OwnerID: 2, Side: Sell, Type: Limit, Price: 100, Qty: 10})
	// 用户 1 买 10 ETH @ 100
	trades, err := r.Place(ctx, eth, Order{ID: 2, OwnerID: 1, Side: Buy, Type: Limit, Price: 100, Qty: 10})
	if err != nil {
		t.Fatalf("ETH/USD 下单报错: %v", err)
	}
	if len(trades) != 1 || trades[0].Qty != 10 {
		t.Errorf("ETH/USD 应成交 10, got %+v", trades)
	}

	// BTC/USD: 用户 3 挂卖 5 BTC @ 5000
	r.Place(ctx, btc, Order{ID: 3, OwnerID: 3, Side: Sell, Type: Limit, Price: 5000, Qty: 5})
	// 用户 1 买 5 BTC @ 5000
	trades2, err := r.Place(ctx, btc, Order{ID: 4, OwnerID: 1, Side: Buy, Type: Limit, Price: 5000, Qty: 5})
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
	l := NewLedger()
	r := NewRouter(l)
	ctx := context.Background()

	_, err := r.Place(ctx, Symbol{Base: "DOGE", Quote: "USD"}, Order{ID: 1})
	if !errors.Is(err, ErrUnknownSymbol) {
		t.Errorf("未注册币对应返回 ErrUnknownSymbol, got %v", err)
	}

	err = r.Cancel(ctx, Symbol{Base: "DOGE", Quote: "USD"}, 1)
	if !errors.Is(err, ErrUnknownSymbol) {
		t.Errorf("未注册币对撤单应返回 ErrUnknownSymbol, got %v", err)
	}
}
