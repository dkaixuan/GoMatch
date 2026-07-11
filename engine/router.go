package engine

import (
	"context"
	"errors"

	"exchange/ledger"
	"exchange/matching"
)

var ErrUnknownSymbol = errors.New("unknown symbol")

// Router 把请求按币对分发到对应的 Engine。
type Router struct {
	engines map[matching.Symbol]*Engine
	ledger  *ledger.Ledger
}

// NewRouter 创建路由, 传入共享的 Ledger。
func NewRouter(l *ledger.Ledger) *Router {
	return &Router{
		engines: make(map[matching.Symbol]*Engine),
		ledger:  l,
	}
}

// Register 注册一个币对, 创建对应的 Engine 并启动 Run。
func (r *Router) Register(ctx context.Context, sym matching.Symbol) {
	e := NewEngineWithLedger(r.ledger, sym.Base, sym.Quote)
	r.engines[sym] = e
	go e.Run(ctx)
}

// Place 下单到指定币对。
func (r *Router) Place(ctx context.Context, sym matching.Symbol, order matching.Order) ([]matching.Trade, error) {
	engine := r.engines[sym]
	if engine == nil {
		return nil, ErrUnknownSymbol
	}
	trades, err := engine.Place(ctx, order)
	return trades, err
}

// Cancel 撤单。
func (r *Router) Cancel(ctx context.Context, sym matching.Symbol, orderID int64) error {
	engine := r.engines[sym]
	if engine == nil {
		return ErrUnknownSymbol
	}
	return engine.Cancel(ctx, orderID)
}

// GetSnapshot 获取某币对的盘口快照。
func (r *Router) GetSnapshot(ctx context.Context, sym matching.Symbol) (matching.BookSnapshot, error) {
	engine := r.engines[sym]
	if engine == nil {
		return matching.BookSnapshot{}, ErrUnknownSymbol
	}
	bookSnapshot, err := engine.GetSnapshot(ctx)
	return bookSnapshot, err
}
