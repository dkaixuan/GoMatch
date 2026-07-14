package store

import (
	"testing"
	"time"
)

// 编译期检查: MemoryStore 满足 TradeStore 接口
var _ TradeStore = (*MemoryStore)(nil)

// 编译期检查: MySQLStore 满足 TradeStore 接口
var _ TradeStore = (*MySQLStore)(nil)

func TestMemoryStoreSaveAndList(t *testing.T) {
	s := NewMemoryStore()

	// 存 3 笔成交
	s.SaveTrade(TradeRecord{Symbol: "ETH/USD", Price: 100, Qty: 10, CreatedAt: time.Now().Add(-2 * time.Second)})
	s.SaveTrade(TradeRecord{Symbol: "ETH/USD", Price: 101, Qty: 5, CreatedAt: time.Now().Add(-1 * time.Second)})
	s.SaveTrade(TradeRecord{Symbol: "BTC/USD", Price: 5000, Qty: 1, CreatedAt: time.Now()})

	// 查 ETH/USD, 应只有 2 笔
	trades, err := s.ListTrades("ETH/USD", 10)
	if err != nil {
		t.Fatalf("ListTrades 报错: %v", err)
	}
	if len(trades) != 2 {
		t.Fatalf("ETH/USD trades = %d, 期望 2", len(trades))
	}

	// 最新的在前面(倒序)
	if trades[0].Price != 101 {
		t.Errorf("第一条 Price = %d, 期望 101(最新)", trades[0].Price)
	}

	// 查 BTC/USD, 应只有 1 笔
	trades2, _ := s.ListTrades("BTC/USD", 10)
	if len(trades2) != 1 {
		t.Errorf("BTC/USD trades = %d, 期望 1", len(trades2))
	}

	// limit 限制
	trades3, _ := s.ListTrades("ETH/USD", 1)
	if len(trades3) != 1 {
		t.Errorf("limit=1 应只返回 1 条, got %d", len(trades3))
	}
}
