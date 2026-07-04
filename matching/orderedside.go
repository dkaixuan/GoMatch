package matching

import "sort"

// OrderedSide 是一侧(买/卖)的有序价格档位集合。
// 内部用 map 做 O(1) 按价格查找, 用有序切片做 O(1) 取最优价。
type OrderedSide struct {
	levels map[int64]*PriceLevel // price → PriceLevel, O(1) 查找
	prices []int64               // 有序价格列表(升序), O(1) 取最优价
}

// NewOrderedSide 创建一个空的有序价格集合。
func NewOrderedSide() *OrderedSide {
	return &OrderedSide{
		levels: make(map[int64]*PriceLevel),
	}
}

// Get 按价格查找档位, O(1)。
func (s *OrderedSide) Get(price int64) *PriceLevel {
	return s.levels[price]
}

// Set 设置一个价格档位。如果是新价格, 插入有序切片, O(log n)。
func (s *OrderedSide) Set(price int64, lvl *PriceLevel) {
	if _, exists := s.levels[price]; !exists {
		// 二分查找插入位置, 保持升序
		i := sort.Search(len(s.prices), func(i int) bool {
			return s.prices[i] >= price
		})
		// 在位置 i 插入
		s.prices = append(s.prices, 0)
		copy(s.prices[i+1:], s.prices[i:])
		s.prices[i] = price
	}
	s.levels[price] = lvl
}

// Delete 删除一个价格档位, O(log n)。
func (s *OrderedSide) Delete(price int64) {
	if _, exists := s.levels[price]; !exists {
		return
	}
	delete(s.levels, price)
	// 从有序切片中移除
	i := sort.Search(len(s.prices), func(i int) bool {
		return s.prices[i] >= price
	})
	if i < len(s.prices) && s.prices[i] == price {
		s.prices = append(s.prices[:i], s.prices[i+1:]...)
	}
}

// Len 返回档位数量。
func (s *OrderedSide) Len() int {
	return len(s.levels)
}

// MaxPrice 返回最高价, O(1)。用于 BestBid。
func (s *OrderedSide) MaxPrice() (int64, bool) {
	if len(s.prices) == 0 {
		return 0, false
	}
	return s.prices[len(s.prices)-1], true
}

// MinPrice 返回最低价, O(1)。用于 BestAsk。
func (s *OrderedSide) MinPrice() (int64, bool) {
	if len(s.prices) == 0 {
		return 0, false
	}
	return s.prices[0], true
}
