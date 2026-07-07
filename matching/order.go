package matching

const (
	Buy Side = iota
	Sell
)

type Side int

const (
	Limit  OrderType = iota // 限价单: 剩余挂上
	Market                  // 市价单: 剩余丢弃, 永不挂上
)

type OrderType int

func (s Side) String() string {
	switch s {
	case Buy:
		return "Buy"
	case Sell:
		return "Sell"
	default:
		return "unknown"
	}
}

type Order struct {
	ID      int64
	OwnerID int64 // 订单所有者, 用于自成交检查
	Side    Side
	Type    OrderType
	Price   int64
	Qty     int64
}

// Trade 表示一笔成交记录。
type Trade struct {
	TakerOrderID int64 // 主动方(吃单方)
	MakerOrderID int64 // 被动方(挂单方)
	BuyerOwnerID  int64 // 买方用户 ID
	SellerOwnerID int64 // 卖方用户 ID
	Price        int64 // 成交价 = maker 的挂单价
	Qty          int64 // 成交量
}
