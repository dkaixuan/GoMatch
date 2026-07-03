package matching

import "errors"

var (
	ErrInvalidQty   = errors.New("订单的数量必须大于0")
	ErrInvalidPrice = errors.New("订单的价格必须大于0")
	ErrNotFound     = errors.New("order not found")
)

func NewOrder(id int64, side Side, price int64, qty int64) (Order, error) {
	if qty <= 0 {
		return Order{}, ErrInvalidQty
	}
	if price <= 0 {
		return Order{}, ErrInvalidPrice
	}
	return Order{ID: id, Side: side, Price: price, Qty: qty}, nil
}
