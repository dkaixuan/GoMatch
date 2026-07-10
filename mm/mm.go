package mm

import "context"

// MarketMaker 是做市 bot 的核心。
type MarketMaker struct {
	Exchange     Exchange // 交易所接口
	SpreadBps    int64    // 价差(基点)
	Qty          int64    // 每笔报价数量
	SkewBps      int64    // 库存偏移系数(基点), 满仓时最大偏移量
	MaxInventory int64    // 最大持仓量, 用于归一化偏移

	liveBidID int64 // 当前活跃买单 ID, 0 表示没有
	liveAskID int64 // 当前活跃卖单 ID, 0 表示没有
	Inventory int64 // 当前持仓量, 正=多头, 负=空头
}

// Quotes 根据参考中间价和当前库存算出买卖报价。
//
// Stage 1 (inventory=0 或 MaxInventory=0): 对称报价, 跟之前一样
// Stage 2 (有库存): 先偏移 center, 再套价差
//
//	skew = SkewBps * inventory / MaxInventory
//	center = mid - mid * skew / 10000
//	bid = center - halfSpread
//	ask = center + halfSpread
//
func (m *MarketMaker) Quotes(mid int64) (bid, ask int64) {
	center := mid
	if m.MaxInventory != 0 {
		skew := m.SkewBps * m.Inventory / m.MaxInventory
		center = mid - mid*skew/10000
	}
	halfSpread := mid * m.SpreadBps / 2 / 10000
	bid = center - halfSpread
	ask = center + halfSpread
	return bid, ask
}

// Step 执行一轮报价:
//  1. 撤掉上一轮的活跃买卖单
//  2. 算新的 Quotes
//  3. 下新买单和卖单
//  4. 记住新单的 ID
func (m *MarketMaker) Step(mid int64) {
	// 撤旧单
	if m.liveBidID != 0 {
		m.Exchange.Cancel(m.liveBidID)
		m.liveBidID = 0
	}
	if m.liveAskID != 0 {
		m.Exchange.Cancel(m.liveAskID)
		m.liveAskID = 0
	}

	// 算报价 + 下新单
	bid, ask := m.Quotes(mid)
	if bid >= ask {
		return
	}

	if m.MaxInventory == 0 || m.Inventory < m.MaxInventory {
		bidID, err := m.Exchange.PlaceLimit("buy", bid, m.Qty)
		if err != nil {
			return
		}
		m.liveBidID = bidID
	}

	if m.MaxInventory == 0 || m.Inventory > -m.MaxInventory {
		askID, err := m.Exchange.PlaceLimit("sell", ask, m.Qty)
		if err != nil {
			return
		}
		m.liveAskID = askID
	}
}

// OnFill 在 bot 的订单成交时调用, 更新库存。
func (m *MarketMaker) OnFill(side string, qty int64) {
	if "buy" == side {
		m.Inventory += qty
	}
	if "sell" == side {
		m.Inventory -= qty
	}
}

// Fill 表示一笔成交通知。
type Fill struct {
	Side string
	Qty  int64
}

// Run 是做市 bot 的主循环。
func (m *MarketMaker) Run(ctx context.Context, mid func() int64, ticker <-chan struct{}, fills <-chan Fill) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker:
			m.Step(mid())
		case f := <-fills:
			m.OnFill(f.Side, f.Qty)
		}
	}
}
