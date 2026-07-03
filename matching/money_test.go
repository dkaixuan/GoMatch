package matching

import "testing"

// TestFloatIsDangerous 演示:为什么交易所的钱绝对不能用 float64(小数)。
//
// 这个测试会"通过"——但它通过的方式恰恰证明了 float64 的危险:
// 在计算机里 0.1 + 0.2 并不精确等于 0.3。
func TestFloatIsDangerous(t *testing.T) {
	var a float64 = 0.1
	var b float64 = 0.2
	sum := a + b // 你以为是 0.3?

	if sum == 0.3 {
		t.Errorf("居然相等?那真出鬼了")
	}

	// %.20f 表示打印 20 位小数,看看 sum 到底是多少。
	// 这行用 t.Logf,只有加 -v 跑测试时才显示。
	t.Logf("0.1 + 0.2 在 float64 里其实 = %.20f （不是 0.3!）", sum)
}

// TestIntMoneyIsExact 对照:用 int64 表示钱,运算永远精确、没有误差。
// 诀窍是:把价格想成"最小报价单位(tick)"或"分",它永远是整数。
func TestIntMoneyIsExact(t *testing.T) {
	var price int64 = 100 // 比如 100 个 tick
	var qty int64 = 3

	cost := price * qty
	if cost != 300 {
		t.Errorf("price*qty = %d, 期望精确的 300", cost)
	}
}
