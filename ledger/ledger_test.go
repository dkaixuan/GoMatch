package ledger

import (
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
//
//	买家: 之前冻了 500 USD → 结算后冻结清零, 得 5 ETH
//	卖家: 之前冻了 5 ETH  → 结算后冻结清零, 得 500 USD
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
