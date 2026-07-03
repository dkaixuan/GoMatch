package matching

type PriceLevel struct {
	Price  int64
	Orders []int64 //订单id
}

func (p *PriceLevel) Add(orderID int64) {
	p.Orders = append(p.Orders, orderID)
}

func (p *PriceLevel) Peek() (int64, bool) {
	if len(p.Orders) == 0 {
		return 0, false
	}
	return p.Orders[0], true
}

func (p *PriceLevel) PopFront() (int64, bool) {
	if len(p.Orders) == 0 {
		return 0, false
	}
	orderId := p.Orders[0]
	p.Orders = p.Orders[1:]
	return orderId, true
}
