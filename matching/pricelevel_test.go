package matching

import "testing"

// TestLevelFIFO 检查一个价位上的订单按"先进先出"排队:
//   - Add 把订单 ID 追加到队尾
//   - Peek 看队首(最早的)但不移除,可以重复看
//   - PopFront 取出并移除队首
//   - 队列为空时,Peek/PopFront 返回 ok=false
//
// 注意这里的 (值, bool) 双返回值写法 —— Go 里非常常见的"comma ok"惯用法,
// 用第二个 bool 表示"取到了没有",替代 Java 里返回 null 或抛 NoSuchElementException。
func TestLevelFIFO(t *testing.T) {
	lvl := &PriceLevel{Price: 100}

	// 1) 空队列:Peek / PopFront 都该返回 ok=false
	if _, ok := lvl.Peek(); ok {
		t.Errorf("空队列 Peek 应返回 ok=false")
	}
	if _, ok := lvl.PopFront(); ok {
		t.Errorf("空队列 PopFront 应返回 ok=false")
	}

	// 2) 入队两笔:101 先到,102 后到
	lvl.Add(101)
	lvl.Add(102)

	// 3) Peek 看队首,应是最早的 101;再 Peek 一次仍是 101(不移除)
	if id, ok := lvl.Peek(); !ok || id != 101 {
		t.Errorf("Peek = (%d, %v), 期望 (101, true)", id, ok)
	}
	if id, ok := lvl.Peek(); !ok || id != 101 {
		t.Errorf("第二次 Peek 应仍是 101(Peek 不能移除元素)")
	}

	// 4) PopFront 取出并移除 101
	if id, ok := lvl.PopFront(); !ok || id != 101 {
		t.Errorf("PopFront = (%d, %v), 期望 (101, true)", id, ok)
	}

	// 5) 现在队首应轮到 102
	if id, ok := lvl.Peek(); !ok || id != 102 {
		t.Errorf("移除 101 后队首应是 102, 实际 (%d, %v)", id, ok)
	}
	if id, ok := lvl.PopFront(); !ok || id != 102 {
		t.Errorf("PopFront 应取出 102, 实际 (%d, %v)", id, ok)
	}

	// 6) 全部取完,又变空
	if _, ok := lvl.PopFront(); ok {
		t.Errorf("取空后 PopFront 应返回 ok=false")
	}
}
