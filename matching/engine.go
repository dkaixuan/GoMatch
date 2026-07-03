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
	book *Book
	cmds chan Command
}

// NewEngine 创建引擎。
func NewEngine() *Engine {
	return &Engine{
		book: NewBook(),
		cmds: make(chan Command, 64), // 带缓冲, 避免发送方阻塞
	}
}

// Run 是引擎的主循环, 应在独立 goroutine 中运行:
//
//	go engine.Run(ctx)
//
// 用 select 监听:
//   - ctx.Done() → 退出
//   - cmds channel → 取出命令, 分派给 book 的方法, 把结果发回 Reply
//
// 你来实现。
func (e *Engine) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case cmd := <-e.cmds:
			if OpPlaceOrder == cmd.Op {
				// 处理下单命令
				submitTrade, err := e.book.Submit(cmd.Order)
				cmd.Reply <- Result{Trades: submitTrade, Error: err}
			}
			if OpCancelOrder == cmd.Op {
				//处理撤单命令
				err := e.book.CancelOrder(cmd.OrderID)
				cmd.Reply <- Result{Error: err}
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
// 你来实现。
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
// 如果 ctx 被取消, 立刻返回 ctx.Err(), 不卡死。
//
// 你来实现。
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
