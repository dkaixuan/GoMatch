package matching

import "sync"

// Event 是事件总线上传递的事件。
type Event struct {
	Type   string      // "trade" 或 "book_update"
	Symbol Symbol      // 哪个币对
	Data   interface{} // 具体数据(Trade 或 BookSnapshot)
}

// EventBus 是一个简单的发布/订阅事件总线。
// 每个订阅者有自己的带缓冲 channel, Publish 非阻塞(满了就丢)。
type EventBus struct {
	mu   sync.RWMutex
	subs map[int]chan Event // subscriberID → channel
	next int               // 下一个订阅者 ID
}

// NewEventBus 创建事件总线。
func NewEventBus() *EventBus {
	return &EventBus{
		subs: make(map[int]chan Event),
	}
}

// Subscribe 注册一个订阅者, 返回 ID 和接收事件的 channel。
func (b *EventBus) Subscribe(bufSize int) (int, <-chan Event) {
	// TODO
	return 0, nil
}

// Unsubscribe 取消订阅, 关闭 channel。
func (b *EventBus) Unsubscribe(id int) {
	// TODO
}

// Publish 向所有订阅者发送事件(非阻塞, 满了就丢)。
func (b *EventBus) Publish(event Event) {
	// TODO
}
