package matching

import "errors"

var ErrInsufficientBalance = errors.New("insufficient balance")

// Account 是一个用户的资金账户。
// 每种资产分"可用"和"冻结"两部分:
//   - 可用: 可以用来下新单
//   - 冻结: 已经被挂单占用, 等待成交或撤单
type Account struct {
	Available map[string]int64 // 资产 → 可用余额, 如 "USD" → 10000
	Frozen    map[string]int64 // 资产 → 冻结余额
}

// Ledger 是所有用户的账本。
type Ledger struct {
	accounts map[int64]*Account // ownerID → 账户
}

// NewLedger 创建空账本。
func NewLedger() *Ledger {
	return &Ledger{
		accounts: make(map[int64]*Account),
	}
}

// account 取或建一个用户的账户(内部用)。
func (l *Ledger) account(owner int64) *Account {
	acc := l.accounts[owner]
	if acc == nil {
		acc = &Account{
			Available: make(map[string]int64),
			Frozen:    make(map[string]int64),
		}
		l.accounts[owner] = acc
	}
	return acc
}

// Deposit 入金: 给用户的某种资产增加可用余额。
//
// 你来实现。
func (l *Ledger) Deposit(owner int64, asset string, amount int64) {
	// TODO: 你来写
}

// Available 查询用户某种资产的可用余额。
//
// 你来实现。
func (l *Ledger) Available(owner int64, asset string) int64 {
	// TODO: 你来写
	return 0
}

// Frozen 查询用户某种资产的冻结余额。
//
// 你来实现。
func (l *Ledger) Frozen(owner int64, asset string) int64 {
	// TODO: 你来写
	return 0
}

// Freeze 冻结: 把 amount 从可用挪到冻结。
// 可用不足时返回 ErrInsufficientBalance, 且不改变任何余额。
//
// 你来实现。
func (l *Ledger) Freeze(owner int64, asset string, amount int64) error {
	// TODO: 你来写
	return nil
}

// Unfreeze 解冻: 把 amount 从冻结挪回可用。
//
// 你来实现。
func (l *Ledger) Unfreeze(owner int64, asset string, amount int64) {
	// TODO: 你来写
}
