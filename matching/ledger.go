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
	account := l.account(owner)
	if amount >= 0 {
		account.Available[asset] += amount
	}
}

// Available 查询用户某种资产的可用余额。
//
// 你来实现。
func (l *Ledger) Available(owner int64, asset string) int64 {
	accounts := l.accounts[owner]
	if accounts == nil {
		return 0
	}
	return accounts.Available[asset]
}

// Frozen 查询用户某种资产的冻结余额。
//
// 你来实现。
func (l *Ledger) Frozen(owner int64, asset string) int64 {
	accounts := l.accounts[owner]
	if accounts == nil {
		return 0
	}
	return accounts.Frozen[asset]
}

// Freeze 冻结: 把 amount 从可用挪到冻结。
// 可用不足时返回 ErrInsufficientBalance, 且不改变任何余额。
//
// 你来实现。
func (l *Ledger) Freeze(owner int64, asset string, amount int64) error {
	account := l.account(owner)
	if account.Available[asset] < amount {
		return ErrInsufficientBalance
	}
	account.Available[asset] -= amount
	account.Frozen[asset] += amount
	return nil
}

// Unfreeze 解冻: 把 amount 从冻结挪回可用。
//
// 你来实现。
func (l *Ledger) Unfreeze(owner int64, asset string, amount int64) {
	account := l.account(owner)
	account.Frozen[asset] -= amount
	account.Available[asset] += amount
}

// Settle 结算一笔成交, 四笔账:
//
//	买家: 冻结 quote(USD) 扣 price*qty, 可用 base(ETH) 加 qty
//	卖家: 冻结 base(ETH) 扣 qty,        可用 quote(USD) 加 price*qty
//
// base = 交易的货(如 "ETH"), quote = 计价货币(如 "USD")
//
// 你来实现。
func (l *Ledger) Settle(buyer, seller int64, base, quote string, price, qty int64) {
	accounts := l.accounts[buyer]
	cost := price * qty
	if accounts == nil {
		panic("buyer account not found")
	}
	sellerAccounts := l.accounts[seller]
	if sellerAccounts == nil {
		panic("seller account not found")
	}

	l.account(buyer).Frozen[quote] -= cost
	l.account(buyer).Available[base] += qty

	l.account(seller).Frozen[base] -= qty
	l.account(seller).Available[quote] += cost
}
