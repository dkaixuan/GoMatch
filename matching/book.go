package matching

import (
	"math"
)

type Book struct {
	bids   map[int64]*PriceLevel
	asks   map[int64]*PriceLevel
	orders map[int64]Order
	locate map[int64]int64 // orderID -> price, O(1) 撤单用
}

func NewBook() *Book {
	return &Book{
		bids:   make(map[int64]*PriceLevel),
		asks:   make(map[int64]*PriceLevel),
		orders: make(map[int64]Order),
		locate: make(map[int64]int64),
	}
}

// AddOrder 把一笔订单加入订单簿:
//   - 存进 orders map
//   - 找到或新建对应价格的 PriceLevel, 追加订单 ID
//   - 记录 locate[orderID] = price
func (b *Book) AddOrder(o Order) {
	b.orders[o.ID] = o
	b.locate[o.ID] = o.Price

	typeSide := b.bids
	if Sell == o.Side {
		typeSide = b.asks
	}

	lvl := typeSide[o.Price]
	if lvl == nil {
		lvl = &PriceLevel{Price: o.Price}
		typeSide[o.Price] = lvl
	}
	lvl.Add(o.ID)
}

// BestBid 返回当前买方最高价(买家愿出的最高价)。
// 簿空时返回 (0, false)。
//
// 你来实现。
func (b *Book) BestBid() (int64, bool) {
	if len(b.bids) == 0 {
		return 0, false
	}
	var bestPrice int64
	for _, bid := range b.bids {
		if bid.Price > bestPrice {
			bestPrice = bid.Price
		}
	}
	return bestPrice, true
}

// BestAsk 返回当前卖方最低价(卖家愿接受的最低价)。
// 簿空时返回 (0, false)。
//
// 你来实现。
func (b *Book) BestAsk() (int64, bool) {
	if len(b.asks) == 0 {
		return 0, false
	}
	var bestPrice int64 = math.MaxInt64 // 初始化为最大整数
	for _, ask := range b.asks {
		if ask.Price < bestPrice {
			bestPrice = ask.Price
		}
	}
	return bestPrice, true
}

// CancelOrder 从订单簿中撤销指定 ID 的订单:
//   - 用 locate 找到订单所在价格 (O(1))
//   - 从该档位的 Orders 切片中删除该 ID (保持剩余顺序不变)
//   - 从 orders、locate 中删除
//   - 如果档位变空, 从 bids/asks map 中删除该档位 (避免幽灵价)
//   - 订单不存在时返回 ErrNotFound
//
// 你来实现。
func (b *Book) CancelOrder(orderID int64) error {
	order := b.orders[orderID]
	price := b.locate[orderID]
	if order.ID == 0 {
		return ErrNotFound
	}

	typeSide := b.bids
	lvl := b.bids[price]
	if Sell == order.Side {
		lvl = b.asks[price]
		typeSide = b.asks
	}
	// 从 PriceLevel 的 Orders 切片中删除该订单 ID
	for i, id := range lvl.Orders {
		if id == orderID {
			lvl.Orders = append(lvl.Orders[:i], lvl.Orders[i+1:]...)
			break
		}
	}
	if len(lvl.Orders) == 0 {
		delete(typeSide, price)
	}

	delete(b.orders, orderID)
	delete(b.locate, orderID)
	return nil
}

// ModifyOrder 修改订单的价格和/或数量, 遵守价格时间优先规则:
//   - 纯减量(价格不变, 新数量 < 旧数量): 原地改 Qty, 保持队列位置
//   - 增量或改价: cancel + re-add, 排到队尾(失去时间优先)
//   - 订单不存在时返回 ErrNotFound
//   - 新数量 <= 0 时返回 ErrInvalidQty
//
// 你来实现。
func (b *Book) ModifyOrder(orderID int64, newPrice int64, newQty int64) error {
	order := b.orders[orderID]
	if order.ID == 0 {
		return ErrNotFound
	}
	if newQty <= 0 {
		return ErrInvalidQty
	}
	if newPrice == order.Price && newQty < order.Qty {
		order.Qty = newQty
	} else {
		order.Qty = newQty
		order.Price = newPrice
		b.CancelOrder(orderID)
		b.AddOrder(order)
	}
	b.orders[orderID] = order
	return nil
}

// Submit 提交一笔订单进行撮合:
//   - 循环: 每轮取最优对手价, 检查交叉, 撮合队首 maker
//   - 直到 taker 无量 或 不再交叉
//   - taker 有剩余则挂上
func (b *Book) Submit(order Order) ([]Trade, error) {
	var trades []Trade
	if order.Qty <= 0 {
		return nil, ErrInvalidQty
	}
	if Limit == order.Type && order.Price <= 0 {
		return nil, ErrInvalidPrice
	}

	var ownerBreak bool

	for order.Qty > 0 {

		// 根据 taker 方向, 取对手最优价
		var bestPrice int64
		var ok bool
		if order.Side == Buy {
			bestPrice, ok = b.BestAsk()
		} else {
			bestPrice, ok = b.BestBid()
		}

		// 没有对手 或 不交叉 → 停
		if !ok {
			break
		}

		if Limit == order.Type {
			if order.Side == Buy && !crosses(order.Price, bestPrice) {
				break
			}
			if order.Side == Sell && !crosses(bestPrice, order.Price) {
				break
			}
		}

		// 取对手档位的队首 maker
		var lvl *PriceLevel
		if order.Side == Buy {
			lvl = b.asks[bestPrice]
		} else {
			lvl = b.bids[bestPrice]
		}
		makerID := lvl.Orders[0]
		maker := b.orders[makerID]

		if order.OwnerID != 0 && order.OwnerID == maker.OwnerID {
			// 自成交检查: taker 和 maker 属于同一用户 → 停止撮合
			ownerBreak = true
			break
		}

		// 成交量 = 两边较小的
		fill := min(order.Qty, maker.Qty)

		// maker 减量或移除
		maker.Qty -= fill
		if maker.Qty <= 0 {
			b.CancelOrder(makerID)
		} else {
			b.orders[makerID] = maker
		}

		// taker 减量
		order.Qty -= fill
		// 记录成交
		trades = append(trades, Trade{
			TakerOrderID: order.ID,
			MakerOrderID: makerID,
			Price:        bestPrice,
			Qty:          fill,
		})
	}

	// taker 还有剩余 → 挂到簿上
	if order.Qty > 0 && order.Type == Limit && !ownerBreak {
		b.AddOrder(order)
	}

	return trades, nil
}

// SnapshotLevel 表示快照中的一个价格档位。
type SnapshotLevel struct {
	Price    int64   `json:"price"`
	TotalQty int64   `json:"total_qty"`
	Orders   []int64 `json:"orders"` // 订单 ID 列表(深拷贝)
}

// BookSnapshot 是订单簿在某一时刻的不可变深拷贝。
type BookSnapshot struct {
	Bids []SnapshotLevel `json:"bids"`
	Asks []SnapshotLevel `json:"asks"`
}

// Snapshot 返回当前簿的深拷贝快照。
// 快照与原簿不共享任何底层数组, 可以安全地在其他 goroutine 中使用。
//
// 你来实现。
func (b *Book) Snapshot() BookSnapshot {
	var bids []SnapshotLevel
	var asks []SnapshotLevel
	for price, priceLevel := range b.asks {
		ordersCopy := make([]int64, len(priceLevel.Orders))
		copy(ordersCopy, priceLevel.Orders)
		var orderQtyTotal int64
		for _, orderID := range priceLevel.Orders {
			orderQtyTotal += b.orders[orderID].Qty
		}
		asks = append(asks, SnapshotLevel{
			Price:    price,
			Orders:   ordersCopy,
			TotalQty: orderQtyTotal,
		})
	}

	for price, priceLevel := range b.bids {
		ordersCopy := make([]int64, len(priceLevel.Orders))
		copy(ordersCopy, priceLevel.Orders)
		var orderQtyTotal int64
		for _, orderID := range priceLevel.Orders {
			orderQtyTotal += b.orders[orderID].Qty
		}
		bids = append(bids, SnapshotLevel{
			Price:    price,
			Orders:   ordersCopy,
			TotalQty: orderQtyTotal,
		})
	}
	return BookSnapshot{Bids: bids, Asks: asks}
}
