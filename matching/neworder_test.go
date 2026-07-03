package matching

import (
	"errors"
	"testing"
)

// TestNewOrder 检查"构造一笔订单时顺便校验合法性"。
// 重点:NewOrder 出错时不是抛异常,而是返回第二个值 error。
// 调用方用 errors.Is 判断这个 error 是不是某个预期的"哨兵错误"。
func TestNewOrder(t *testing.T) {
	cases := []struct {
		name    string
		id      int64
		side    Side
		price   int64
		qty     int64
		wantErr error // nil 表示"期望成功,不报错"
	}{
		{name: "合法订单", id: 1, side: Buy, price: 100, qty: 10, wantErr: nil},
		{name: "数量为0非法", id: 2, side: Buy, price: 100, qty: 0, wantErr: ErrInvalidQty},
		{name: "数量为负非法", id: 3, side: Sell, price: 100, qty: -5, wantErr: ErrInvalidQty},
		{name: "价格为0非法", id: 4, side: Buy, price: 0, qty: 10, wantErr: ErrInvalidPrice},
		{name: "价格为负非法", id: 5, side: Sell, price: -1, qty: 10, wantErr: ErrInvalidPrice},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			o, err := NewOrder(c.id, c.side, c.price, c.qty)

			if c.wantErr != nil {
				// 期望失败:err 应当匹配预期的哨兵错误。
				if !errors.Is(err, c.wantErr) {
					t.Errorf("NewOrder(...) err = %v, 期望 %v", err, c.wantErr)
				}
				return // 这个用例验完了,提前结束
			}

			// 期望成功:err 必须是 nil,且字段和输入对得上。
			if err != nil {
				t.Errorf("NewOrder(...) 意外报错: %v", err)
			}
			if o.ID != c.id || o.Side != c.side || o.Price != c.price || o.Qty != c.qty {
				t.Errorf("NewOrder(...) = %+v, 字段与输入不符", o)
			}
		})
	}
}
