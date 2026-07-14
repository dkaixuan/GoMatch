package store

import "sync"

// MemoryStore 是 TradeStore 的内存实现, 用于测试。
type MemoryStore struct {
	mu     sync.Mutex
	trades []TradeRecord
}

// NewMemoryStore 创建内存存储。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{}
}

// SaveTrade 保存一笔成交记录到内存。
//
// 你来实现。
func (m *MemoryStore) SaveTrade(record TradeRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.trades = append(m.trades, record)
	return nil
}

// ListTrades 按币对查询最近的成交记录。
//
// 你来实现。
func (m *MemoryStore) ListTrades(symbol string, limit int) ([]TradeRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var result []TradeRecord
	for i := len(m.trades) - 1; i >= 0; i-- {
		if m.trades[i].Symbol == symbol {
			result = append(result, m.trades[i])
			if len(result) >= limit {
				break
			}
		}
	}
	return result, nil
}
