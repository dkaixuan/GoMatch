package matching

import "testing"

// TestSideString 检查 Side 能打印成人类可读的字样。
// 这就要求 Side 有一个 String() 方法(实现了标准库的 fmt.Stringer 接口)。
func TestSideString(t *testing.T) {
	cases := []struct {
		side Side
		want string
	}{
		{Buy, "Buy"},
		{Sell, "Sell"},
	}
	for _, c := range cases {
		t.Run(c.want, func(t *testing.T) {
			if got := c.side.String(); got != c.want {
				t.Errorf("Side(%d).String() = %q, 期望 %q", c.side, got, c.want)
			}
		})
	}
}

// TestOrderLiteral 用一个结构体字面量造出一笔订单,并断言各字段读得回来。
// 这就要求 Order 结构体必须有 ID / Side / Price / Qty 这几个字段,且类型对得上。
func TestOrderLiteral(t *testing.T) {
	o := Order{ID: 1, Side: Buy, Price: 100, Qty: 10}

	if o.ID != 1 {
		t.Errorf("ID = %d, 期望 1", o.ID)
	}
	if o.Side != Buy {
		t.Errorf("Side = %v, 期望 Buy", o.Side)
	}
	if o.Price != 100 {
		t.Errorf("Price = %d, 期望 100", o.Price)
	}
	if o.Qty != 10 {
		t.Errorf("Qty = %d, 期望 10", o.Qty)
	}
}