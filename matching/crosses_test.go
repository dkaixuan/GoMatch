package matching

import "testing"

// TestCrosses 检验整个交易所里最核心的一个判断:
// 一笔买价 buy 的买单,能不能和一笔卖价 sell 的挂单"成交(交叉)"?
// 规则:只有买家愿意出的价 >= 卖家要的价,才可能成交。
//
// 这是一个"表驱动测试"(table-driven test)—— Go 里地道的写法,
// 用来替代你在 JUnit 里写一堆几乎一样的 @Test 方法:
//   一个用例切片 cases、一个 for 循环、每个用例一个 t.Run 子测试,
//   这样哪一行挂了,失败信息会精确告诉你是哪一行。
//
// 注意:这里没有 @Test 注解、没有 assertEquals、没有任何框架。
// 一个名字以 Test 开头、参数是 *testing.T 的函数,就是一个测试。
func TestCrosses(t *testing.T) {
	cases := []struct {
		name string
		buy  int64
		sell int64
		want bool
	}{
		{name: "价相等可成交", buy: 100, sell: 100, want: true},
		{name: "买价高于卖价可成交", buy: 101, sell: 100, want: true},
		{name: "买价低于卖价不成交", buy: 99, sell: 100, want: false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := crosses(c.buy, c.sell)
			if got != c.want {
				// t.Errorf 标记本子测试失败但继续跑其余用例(对比 t.Fatalf 会立刻停)。
				t.Errorf("crosses(%d, %d) = %v, 期望 %v", c.buy, c.sell, got, c.want)
			}
		})
	}
}
