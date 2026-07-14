package store

import "time"

// TradeRecord 是存入数据库的成交记录。
type TradeRecord struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	TakerOrderID  int64     `json:"taker_order_id"`
	MakerOrderID  int64     `json:"maker_order_id"`
	BuyerOwnerID  int64     `json:"buyer_owner_id"`
	SellerOwnerID int64     `json:"seller_owner_id"`
	Symbol        string    `json:"symbol"`
	Price         int64     `json:"price"`
	Qty           int64     `json:"qty"`
	CreatedAt     time.Time `json:"created_at"`
}

// TradeStore 是成交记录的存储接口。
// 用接口隔离数据库: 测试用内存实现, 生产用 MySQL。
type TradeStore interface {
	SaveTrade(record TradeRecord) error
	ListTrades(symbol string, limit int) ([]TradeRecord, error)
}
