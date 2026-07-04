package mm

// fakeExchange 是一个记录所有操作的假交易所, 用于测试 bot 逻辑。
// 不需要真正的撮合, 只记录 bot 下了什么单、撤了什么单。
type fakeExchange struct {
	nextID   int64
	placed   []fakePlaced  // 记录所有下单
	canceled []int64       // 记录所有撤单 ID
	bestBid  int64
	bestAsk  int64
	hasBid   bool
	hasAsk   bool
}

type fakePlaced struct {
	ID    int64
	Side  string
	Price int64
	Qty   int64
}

func newFakeExchange() *fakeExchange {
	return &fakeExchange{}
}

func (f *fakeExchange) PlaceLimit(side string, price, qty int64) (int64, error) {
	f.nextID++
	f.placed = append(f.placed, fakePlaced{
		ID: f.nextID, Side: side, Price: price, Qty: qty,
	})
	return f.nextID, nil
}

func (f *fakeExchange) Cancel(id int64) error {
	f.canceled = append(f.canceled, id)
	return nil
}

func (f *fakeExchange) BestBid() (int64, bool) {
	return f.bestBid, f.hasBid
}

func (f *fakeExchange) BestAsk() (int64, bool) {
	return f.bestAsk, f.hasAsk
}
