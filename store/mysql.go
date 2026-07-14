package store

import (
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// MySQLStore 是 TradeStore 的 MySQL 实现。
type MySQLStore struct {
	db *gorm.DB
}

// NewMySQLStore 连接 MySQL 并自动建表。
// dsn 格式: "root:gomatch123@tcp(127.0.0.1:3306)/gomatch?charset=utf8mb4&parseTime=True"
func NewMySQLStore(dsn string) (*MySQLStore, error) {
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, err
	}
	// 自动建表
	db.AutoMigrate(&TradeRecord{})
	return &MySQLStore{db: db}, nil
}

// SaveTrade 保存一笔成交记录到 MySQL。
func (s *MySQLStore) SaveTrade(record TradeRecord) error {
	return s.db.Create(&record).Error
}

// ListTrades 按币对查询最近的成交记录, 按时间倒序。
func (s *MySQLStore) ListTrades(symbol string, limit int) ([]TradeRecord, error) {
	var trades []TradeRecord
	err := s.db.Where("symbol = ?", symbol).
		Order("created_at DESC").
		Limit(limit).
		Find(&trades).Error
	return trades, err
}
