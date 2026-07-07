package matching

import (
	"context"
	"errors"
	"testing"
)

// TestLedgerDeposit 检查入金和查询。
func TestLedgerDeposit(t *testing.T) {
	l := NewLedger()

	// 新用户余额为 0
	if got := l.Available(1, "USD"); got != 0 {
		t.Errorf("新用户 Available = %d, 期望 0", got)
	}

	// 入金 10000
	l.Deposit(1, "USD", 10000)
	if got := l.Available(1, "USD"); got != 10000 {
		t.Errorf("入金后 Available = %d, 期望 10000", got)
	}

	// 再入金 5000, 累加
	l.Deposit(1, "USD", 5000)
	if got := l.Available(1, "USD"); got != 15000 {
		t.Errorf("再次入金后 Available = %d, 期望 15000", got)
	}

	// 不同资产互不影响
	l.Deposit(1, "ETH", 5)
	if got := l.Available(1, "ETH"); got != 5 {
		t.Errorf("ETH Available = %d, 期望 5", got)
	}
	if got := l.Available(1, "USD"); got != 15000 {
		t.Errorf("USD 不应受 ETH 入金影响, got %d", got)
	}

	// 不同用户互不影响
	if got := l.Available(2, "USD"); got != 0 {
		t.Errorf("用户 2 Available = %d, 期望 0", got)
	}
}

// TestLedgerFreeze 检查冻结/解冻。
func TestLedgerFreeze(t *testing.T) {
	l := NewLedger()
	l.Deposit(1, "USD", 1000)

	// 冻结 600: 可用 400, 冻结 600
	if err := l.Freeze(1, "USD", 600); err != nil {
		t.Fatalf("Freeze 报错: %v", err)
	}
	if got := l.Available(1, "USD"); got != 400 {
		t.Errorf("冻结后 Available = %d, 期望 400", got)
	}
	if got := l.Frozen(1, "USD"); got != 600 {
		t.Errorf("冻结后 Frozen = %d, 期望 600", got)
	}

	// 再冻结 500: 可用只剩 400, 不够 → 报错, 余额不变
	err := l.Freeze(1, "USD", 500)
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Errorf("超额冻结应返回 ErrInsufficientBalance, got %v", err)
	}
	if got := l.Available(1, "USD"); got != 400 {
		t.Errorf("失败的冻结不应改变 Available, got %d", got)
	}
	if got := l.Frozen(1, "USD"); got != 600 {
		t.Errorf("失败的冻结不应改变 Frozen, got %d", got)
	}

	// 解冻 200: 可用 600, 冻结 400
	l.Unfreeze(1, "USD", 200)
	if got := l.Available(1, "USD"); got != 600 {
		t.Errorf("解冻后 Available = %d, 期望 600", got)
	}
	if got := l.Frozen(1, "USD"); got != 400 {
		t.Errorf("解冻后 Frozen = %d, 期望 400", got)
	}
}

// TestLedgerConservation 检查守恒: 无论怎么冻结/解冻, 总额不变。
func TestLedgerConservation(t *testing.T) {
	l := NewLedger()
	l.Deposit(1, "USD", 1000)

	l.Freeze(1, "USD", 300)
	l.Freeze(1, "USD", 200)
	l.Unfreeze(1, "USD", 100)
	l.Freeze(1, "USD", 400)

	total := l.Available(1, "USD") + l.Frozen(1, "USD")
	if total != 1000 {
		t.Errorf("总额 = %d, 期望 1000 (冻结/解冻不该改变总额)", total)
	}
}

// TestSettle 检查成交结算的四笔账。
//
// 场景: 买家(1)买 5 ETH @ 100, 卖家(2)卖。
//   买家: 之前冻了 500 USD → 结算后冻结清零, 得 5 ETH
//   卖家: 之前冻了 5 ETH  → 结算后冻结清零, 得 500 USD
func TestSettle(t *testing.T) {
	l := NewLedger()

	// 买家有 1000 USD, 下单时冻结了 500
	l.Deposit(1, "USD", 1000)
	l.Freeze(1, "USD", 500)

	// 卖家有 10 ETH, 挂卖单时冻结了 5
	l.Deposit(2, "ETH", 10)
	l.Freeze(2, "ETH", 5)

	// 成交: 5 ETH @ 100
	l.Settle(1, 2, "ETH", "USD", 100, 5)

	// 买家: 冻结 USD 500-500=0, 可用 USD 还是 500, 可用 ETH +5
	if got := l.Frozen(1, "USD"); got != 0 {
		t.Errorf("买家冻结 USD = %d, 期望 0", got)
	}
	if got := l.Available(1, "USD"); got != 500 {
		t.Errorf("买家可用 USD = %d, 期望 500", got)
	}
	if got := l.Available(1, "ETH"); got != 5 {
		t.Errorf("买家可用 ETH = %d, 期望 5", got)
	}

	// 卖家: 冻结 ETH 5-5=0, 可用 ETH 还是 5, 可用 USD +500
	if got := l.Frozen(2, "ETH"); got != 0 {
		t.Errorf("卖家冻结 ETH = %d, 期望 0", got)
	}
	if got := l.Available(2, "ETH"); got != 5 {
		t.Errorf("卖家可用 ETH = %d, 期望 5", got)
	}
	if got := l.Available(2, "USD"); got != 500 {
		t.Errorf("卖家可用 USD = %d, 期望 500", got)
	}

	// 守恒: 全市场 USD 总量 1000, ETH 总量 10, 结算前后不变
	usdTotal := l.Available(1, "USD") + l.Frozen(1, "USD") +
		l.Available(2, "USD") + l.Frozen(2, "USD")
	ethTotal := l.Available(1, "ETH") + l.Frozen(1, "ETH") +
		l.Available(2, "ETH") + l.Frozen(2, "ETH")
	if usdTotal != 1000 {
		t.Errorf("全市场 USD = %d, 期望 1000 (结算不该创造/销毁钱)", usdTotal)
	}
	if ethTotal != 10 {
		t.Errorf("全市场 ETH = %d, 期望 10", ethTotal)
	}
}

// TestSubmitWithLedger 检查资金账户与撮合引擎的完整集成:
//   余额不足 → 拒单
//   正常成交 → 双方余额正确
//   撤单 → 解冻
func TestSubmitWithLedger(t *testing.T) {
	l := NewLedger()
	e := NewEngineWithLedger(l, "ETH", "USD")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go e.Run(ctx)

	// --- 准备: 给两个用户入金 ---
	l.Deposit(1, "USD", 10000) // 用户 1: 10000 USD (买方)
	l.Deposit(2, "ETH", 100)   // 用户 2: 100 ETH (卖方)

	// --- 测试 1: 余额不足被拒 ---
	_, err := e.Place(ctx, Order{
		ID: 99, OwnerID: 1, Side: Buy, Type: Limit, Price: 200, Qty: 100,
		// 需要 200*100=20000 USD, 但只有 10000
	})
	if !errors.Is(err, ErrInsufficientBalance) {
		t.Fatalf("余额不足应拒单, got err=%v", err)
	}
	// 被拒后余额不变
	if got := l.Available(1, "USD"); got != 10000 {
		t.Errorf("拒单后 USD 应不变, got %d", got)
	}

	// --- 测试 2: 正常成交 ---
	// 用户 2 挂卖单: 10 ETH @ 100
	_, err = e.Place(ctx, Order{
		ID: 1, OwnerID: 2, Side: Sell, Type: Limit, Price: 100, Qty: 10,
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
	trades, err := e.Place(ctx, Order{
		ID: 2, OwnerID: 1, Side: Buy, Type: Limit, Price: 100, Qty: 10,
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
	_, err = e.Place(ctx, Order{
		ID: 3, OwnerID: 1, Side: Buy, Type: Limit, Price: 50, Qty: 10,
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
