package matching

type Book struct {
	bids   *OrderedSide
	asks   *OrderedSide
	orders map[int64]Order
	locate map[int64]int64 // orderID -> price, O(1) 撤单用
}

func NewBook() *Book {
	return &Book{
		bids:   NewOrderedSide(),
		asks:   NewOrderedSide(),
		orders: make(map[int64]Order),
		locate: make(map[int64]int64),
	}
}

// AddOrder 把一笔订单加入订单簿。
func (b *Book) AddOrder(o Order) {
	b.orders[o.ID] = o
	b.locate[o.ID] = o.Price

	side := b.bids
	if Sell == o.Side {
		side = b.asks
	}

	lvl := side.Get(o.Price)
	if lvl == nil {
		lvl = &PriceLevel{Price: o.Price}
		side.Set(o.Price, lvl)
	}
	lvl.Add(o.ID)
}

// BestBid 返回当前买方最高价。O(1)。
func (b *Book) BestBid() (int64, bool) {
	return b.bids.MaxPrice()
}

// BestAsk 返回当前卖方最低价。O(1)。
func (b *Book) BestAsk() (int64, bool) {
	return b.asks.MinPrice()
}

// CancelOrder 从订单簿中撤销指定 ID 的订单。
func (b *Book) CancelOrder(orderID int64) error {
	order := b.orders[orderID]
	price := b.locate[orderID]
	if order.ID == 0 {
		return ErrNotFound
	}

	side := b.bids
	if Sell == order.Side {
		side = b.asks
	}

	lvl := side.Get(price)
	for i, id := range lvl.Orders {
		if id == orderID {
			lvl.Orders = append(lvl.Orders[:i], lvl.Orders[i+1:]...)
			break
		}
	}
	if len(lvl.Orders) == 0 {
		side.Delete(price)
	}

	delete(b.orders, orderID)
	delete(b.locate, orderID)
	return nil
}

// ModifyOrder 修改订单的价格和/或数量。
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

// Submit 提交一笔订单进行撮合。
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
		var bestPrice int64
		var ok bool
		if order.Side == Buy {
			bestPrice, ok = b.BestAsk()
		} else {
			bestPrice, ok = b.BestBid()
		}

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

		var lvl *PriceLevel
		if order.Side == Buy {
			lvl = b.asks.Get(bestPrice)
		} else {
			lvl = b.bids.Get(bestPrice)
		}
		makerID := lvl.Orders[0]
		maker := b.orders[makerID]

		if order.OwnerID != 0 && order.OwnerID == maker.OwnerID {
			ownerBreak = true
			break
		}

		fill := min(order.Qty, maker.Qty)

		maker.Qty -= fill
		if maker.Qty <= 0 {
			b.CancelOrder(makerID)
		} else {
			b.orders[makerID] = maker
		}

		order.Qty -= fill
		trades = append(trades, Trade{
			TakerOrderID: order.ID,
			MakerOrderID: makerID,
			Price:        bestPrice,
			Qty:          fill,
		})
	}

	if order.Qty > 0 && order.Type == Limit && !ownerBreak {
		b.AddOrder(order)
	}

	return trades, nil
}

// SnapshotLevel 表示快照中的一个价格档位。
type SnapshotLevel struct {
	Price    int64   `json:"price"`
	TotalQty int64   `json:"total_qty"`
	Orders   []int64 `json:"orders"`
}

// BookSnapshot 是订单簿在某一时刻的不可变深拷贝。
type BookSnapshot struct {
	Bids []SnapshotLevel `json:"bids"`
	Asks []SnapshotLevel `json:"asks"`
}

// Snapshot 返回当前簿的深拷贝快照。
func (b *Book) Snapshot() BookSnapshot {
	var bids []SnapshotLevel
	var asks []SnapshotLevel

	for _, price := range b.asks.prices {
		priceLevel := b.asks.Get(price)
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

	for _, price := range b.bids.prices {
		priceLevel := b.bids.Get(price)
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
