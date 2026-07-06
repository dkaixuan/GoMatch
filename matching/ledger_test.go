package matching

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
