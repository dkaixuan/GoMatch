package mm

// Exchange 是做市 bot 对交易所的最小需求。
// 在消费方(bot)这边定义, 不在交易所那边。
// 任何满足这四个方法的类型自动实现该接口, 不用写 implements。
type Exchange interface {
	PlaceLimit(side string, price, qty int64) (int64, error) // 下限价单, 返回订单 ID
	Cancel(id int64) error                                   // 撤单
	BestBid() (int64, bool)                                  // 当前最高买价
	BestAsk() (int64, bool)                                  // 当前最低卖价
}
