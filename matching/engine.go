package matching

import "context"

// OpType 表示命令类型
type OpType int

const (
	OpPlaceOrder   OpType = iota // 下单(Submit)
	OpCancelOrder                // 撤单
	OpGetSnapshot                // 获取快照
)

// Command 是发给引擎的一条命令。
// Reply channel 用于接收引擎处理完后的结果。
type Command struct {
	Op      OpType
	Order   Order // OpPlaceOrder 用
	OrderID int64 // OpCancelOrder 用
	Reply   chan Result
}

// Result 是引擎返回的处理结果。
type Result struct {
	Trades   []Trade
	Snapshot BookSnapshot
	Error    error
}

// Engine 用一个 goroutine 独占 Book, 通过 channel 接收命令。
type Engine struct {
	book   *Book
	cmds   chan Command
	ledger *Ledger // 可选, nil 表示不做余额检查
	base   string  // 如 "ETH"
	quote  string  // 如 "USD"
}

// NewEngine 创建引擎(无 Ledger, 向后兼容)。
func NewEngine() *Engine {
	return &Engine{
		book: NewBook(),
		cmds: make(chan Command, 64),
	}
}

// NewEngineWithLedger 创建带资金账户的引擎。
func NewEngineWithLedger(ledger *Ledger, base, quote string) *Engine {
	return &Engine{
		book:   NewBook(),
		cmds:   make(chan Command, 64),
		ledger: ledger,
		base:   base,
		quote:  quote,
	}
}

// Run 是引擎的主循环, 应在独立 goroutine 中运行:
//
//	go engine.Run(ctx)
//
// Run 是引擎的主循环, 应在独立 goroutine 中运行。
func (e *Engine) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-e.cmds:
			if OpPlaceOrder == cmd.Op {
				e.handlePlace(cmd)
			}
			if OpCancelOrder == cmd.Op {
				e.handleCancel(cmd)
			}
			if OpGetSnapshot == cmd.Op {
				snap := e.book.Snapshot()
				cmd.Reply <- Result{Snapshot: snap}
			}
		}
	}
}

// Place 提交一笔订单, 同步等待结果。
// 如果 ctx 被取消, 立刻返回 ctx.Err(), 不卡死。
//
// Place 提交一笔订单, 同步等待结果。
func (e *Engine) Place(ctx context.Context, order Order) ([]Trade, error) {
	reply := make(chan Result, 1)

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case e.cmds <- Command{Op: OpPlaceOrder, Order: order, OrderID: order.ID, Reply: reply}:
	}

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case res := <-reply:
		return res.Trades, res.Error
	}
}

// Cancel 撤销一笔订单, 同步等待结果。
// Cancel 撤销一笔订单, 同步等待结果。
func (e *Engine) Cancel(ctx context.Context, orderID int64) error {
	reply := make(chan Result, 1)
	select {
	case <-ctx.Done():
		return ctx.Err()
	case e.cmds <- Command{Op: OpCancelOrder, OrderID: orderID, Reply: reply}:
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case res := <-reply:
		return res.Error
	}
}

// GetSnapshot 获取订单簿快照, 同步等待结果。
func (e *Engine) GetSnapshot(ctx context.Context) (BookSnapshot, error) {
	reply := make(chan Result, 1)
	select {
	case <-ctx.Done():
		return BookSnapshot{}, ctx.Err()
	case e.cmds <- Command{Op: OpGetSnapshot, Reply: reply}:
	}
	select {
	case <-ctx.Done():
		return BookSnapshot{}, ctx.Err()
	case res := <-reply:
		return res.Snapshot, res.Error
	}
}

// handlePlace 处理下单: 冻结 → Submit → 结算 → 退还多冻的。
func (e *Engine) handlePlace(cmd Command) {
	order := cmd.Order

	// 1. 冻结资金(如果有 Ledger)
	if e.ledger != nil && order.Type == Limit {
		var freezeAsset string
		var freezeAmount int64
		if order.Side == Buy {
			freezeAsset = e.quote
			freezeAmount = order.Price * order.Qty
		} else {
			freezeAsset = e.base
			freezeAmount = order.Qty
		}
		if err := e.ledger.Freeze(order.OwnerID, freezeAsset, freezeAmount); err != nil {
			cmd.Reply <- Result{Error: err}
			return
		}
	}

	// 2. 撮合
	trades, err := e.book.Submit(order)
	if err != nil {
		if e.ledger != nil && order.Type == Limit {
			if order.Side == Buy {
				e.ledger.Unfreeze(order.OwnerID, e.quote, order.Price*order.Qty)
			} else {
				e.ledger.Unfreeze(order.OwnerID, e.base, order.Qty)
			}
		}
		cmd.Reply <- Result{Error: err}
		return
	}

	// 3. 每笔成交 → 结算
	if e.ledger != nil {
		for _, trade := range trades {
			e.ledger.Settle(trade.BuyerOwnerID, trade.SellerOwnerID,
				e.base, e.quote, trade.Price, trade.Qty)
		}

		// 4. 买单价格改善退款(成交价 < 限价 → 多冻了)
		if order.Side == Buy && order.Type == Limit {
			for _, trade := range trades {
				improvement := (order.Price - trade.Price) * trade.Qty
				if improvement > 0 {
					e.ledger.Unfreeze(order.OwnerID, e.quote, improvement)
				}
			}
		}
	}

	cmd.Reply <- Result{Trades: trades}
}

// handleCancel 处理撤单: 查订单 → 从 book 删 → 解冻。
func (e *Engine) handleCancel(cmd Command) {
	order, exists := e.book.GetOrder(cmd.OrderID)

	err := e.book.CancelOrder(cmd.OrderID)
	if err != nil {
		cmd.Reply <- Result{Error: err}
		return
	}

	if e.ledger != nil && exists {
		if order.Side == Buy {
			e.ledger.Unfreeze(order.OwnerID, e.quote, order.Price*order.Qty)
		} else {
			e.ledger.Unfreeze(order.OwnerID, e.base, order.Qty)
		}
	}

	cmd.Reply <- Result{}
}
