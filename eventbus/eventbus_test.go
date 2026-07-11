package eventbus

import (
	"testing"
	"time"
)

// TestEventBusSubscribePublish 检查订阅后能收到事件。
func TestEventBusSubscribePublish(t *testing.T) {
	bus := NewEventBus()

	id, ch := bus.Subscribe(10)
	_ = id

	bus.Publish(Event{Type: "trade", Data: "hello"})

	select {
	case e := <-ch:
		if e.Type != "trade" || e.Data != "hello" {
			t.Errorf("收到 %+v, 期望 {trade, hello}", e)
		}
	case <-time.After(time.Second):
		t.Fatal("超时, 没收到事件")
	}
}

// TestEventBusUnsubscribe 检查取消订阅后收不到事件。
func TestEventBusUnsubscribe(t *testing.T) {
	bus := NewEventBus()

	id, ch := bus.Subscribe(10)
	bus.Unsubscribe(id)

	bus.Publish(Event{Type: "trade"})

	// channel 应该已关闭, 读取返回零值和 false
	select {
	case _, ok := <-ch:
		if ok {
			t.Error("取消订阅后不应收到事件")
		}
	case <-time.After(100 * time.Millisecond):
		// 也可以, 没收到就对了
	}
}

// TestEventBusSlowConsumer 检查慢消费者不阻塞发布者。
func TestEventBusSlowConsumer(t *testing.T) {
	bus := NewEventBus()

	// 缓冲只有 2
	_, ch := bus.Subscribe(2)

	// 发 10 条, 不应阻塞
	done := make(chan struct{})
	go func() {
		for i := 0; i < 10; i++ {
			bus.Publish(Event{Type: "trade", Data: i})
		}
		close(done)
	}()

	select {
	case <-done:
		// 发布者没被阻塞 ✅
	case <-time.After(time.Second):
		t.Fatal("Publish 被慢消费者阻塞了!")
	}

	// 只能收到缓冲里的(最多 2 条), 其余被丢了
	count := 0
	for {
		select {
		case <-ch:
			count++
		default:
			goto out
		}
	}
out:
	if count > 2 {
		t.Errorf("缓冲 2 但收到 %d 条, 不应超过缓冲大小", count)
	}
}

// TestEventBusMultipleSubscribers 检查多个订阅者都能收到同一事件。
func TestEventBusMultipleSubscribers(t *testing.T) {
	bus := NewEventBus()

	_, ch1 := bus.Subscribe(10)
	_, ch2 := bus.Subscribe(10)

	bus.Publish(Event{Type: "trade", Data: "x"})

	for i, ch := range []<-chan Event{ch1, ch2} {
		select {
		case e := <-ch:
			if e.Data != "x" {
				t.Errorf("订阅者 %d 收到 %v, 期望 x", i, e.Data)
			}
		case <-time.After(time.Second):
			t.Fatalf("订阅者 %d 超时", i)
		}
	}
}
