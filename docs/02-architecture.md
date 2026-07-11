# GoMatch 架构与核心流程梳理

## 1. 整体架构

```
┌─────────────────────────────────────────────────────┐
│                    浏览器 / curl                      │
│              index.html (WebSocket + HTTP)            │
└──────────┬──────────────────────┬────────────────────┘
           │ HTTP                 │ WebSocket
           ▼                      ▼
┌─────────────────────────────────────────────────────┐
│                  Gin Handler                         │
│  POST /orders    DELETE /orders/:id    GET /book      │
│  GET /ws (WebSocket 长连接)                           │
│                                                      │
│  职责: 解析请求 → 调 Engine 方法 → 返回 JSON          │
│  特点: 薄、无状态、每个请求一个 goroutine              │
└──────────┬──────────────────────┬────────────────────┘
           │ e.Place/Cancel       │ bus.Subscribe
           ▼                      │
┌──────────────────────┐          │
│   Router (M7)        │          │
│                      │          │
│  engines: {          │          │
│    ETH/USD → Engine  │          │
│    BTC/USD → Engine  │          │
│  }                   │          │
│                      │          │
│  按 Symbol 分发请求   │          │
└──────────┬───────────┘          │
           │                      │
           ▼                      │
┌──────────────────────┐    ┌─────┴──────────┐
│   Engine (M3)        │    │  EventBus (M8) │
│                      │    │                │
│  单 goroutine Run()  │───▶│  Publish()     │
│  for-select 循环     │    │  Subscribe()   │
│                      │    │  Unsubscribe() │
│  cmds channel 接收   │    └────────────────┘
│  command, 串行处理    │
│                      │
│  handlePlace:        │
│    冻结→撮合→结算     │
│  handleCancel:       │
│    查单→删单→解冻     │
└──────┬───────┬───────┘
       │       │
       ▼       ▼
┌──────────┐ ┌──────────┐
│ Book(M1) │ │Ledger(M6)│
│          │ │          │
│ 订单簿    │ │ 资金账本  │
│ 撮合逻辑  │ │ 冻结/结算 │
│ Submit() │ │ Freeze() │
│ Cancel() │ │ Settle() │
└──────────┘ └──────────┘
```

## 2. 下单完整流程

以 "用户1 买 10 ETH @ 100" 为例, 追踪请求从浏览器到成交的每一步:

```
浏览器
  │
  │  POST /orders {"id":2, "owner_id":1, "side":"buy", "price":100, "qty":10}
  ▼
Gin Handler (goroutine A)
  │  解析 JSON → 构造 Order
  │  调 e.Place(ctx, order)
  │    → 构造 Command{Op:PlaceOrder, Order, Reply:make(chan Result,1)}
  │    → select: 发 Command 到 cmds channel
  │    → select: 等 Reply channel 的结果
  ▼
Engine.Run (goroutine B, 唯一碰 Book 的)
  │
  │  从 cmds channel 取出 Command
  │  进入 handlePlace:
  │
  │  ① 冻结资金
  │     Buy 单 → 冻结 price*qty = 1000 USD
  │     ledger.Freeze(ownerID=1, "USD", 1000)
  │     可用 USD: 10000 → 9000, 冻结 USD: 0 → 1000
  │     如果余额不足 → 返回 ErrInsufficientBalance, 结束
  │
  │  ② 撮合
  │     book.Submit(order)
  │     │
  │     │  for order.Qty > 0 {
  │     │    bestAsk = BestAsk() → 从 OrderedSide 取 O(1)
  │     │    不交叉? → break
  │     │    取队首 maker (FIFO)
  │     │    fill = min(taker.Qty, maker.Qty)
  │     │    maker 减量或删除
  │     │    taker 减量
  │     │    trades = append(trades, Trade{...})
  │     │  }
  │     │  taker 有剩余且是限价单? → AddOrder 挂上
  │     │
  │     └─ 返回 []Trade
  │
  │  ③ 结算 (对每笔 Trade)
  │     ledger.Settle(buyerOwner, sellerOwner, "ETH", "USD", price, qty)
  │       买家: 冻结 USD -= price*qty, 可用 ETH += qty
  │       卖家: 冻结 ETH -= qty,       可用 USD += price*qty
  │
  │  ④ 退价格改善 (如果成交价 < 限价)
  │     improvement = (limitPrice - tradePrice) * qty
  │     ledger.Unfreeze(owner, "USD", improvement)
  │
  │  ⑤ 发布事件
  │     bus.Publish(Event{Type:"trade", Data:trade})
  │     bus.Publish(Event{Type:"book_update", Data:snapshot})
  │
  │  cmd.Reply ← Result{Trades}
  ▼
Gin Handler (goroutine A, 之前一直在等 Reply)
  │  收到 Result
  │  c.JSON(201, trades)
  ▼
浏览器收到响应

同时, EventBus 推送:
  │
  ▼
WebSocket Handler (goroutine C, 每个连接一个)
  │  从 Subscribe 的 channel 收到 Event
  │  conn.WriteMessage(JSON)
  ▼
浏览器 WebSocket.onmessage
  │  更新盘口 / 成交记录
```

## 3. 撤单完整流程

```
DELETE /orders/1
  │
  ▼
Gin Handler
  │  解析 ID
  │  调 e.Cancel(ctx, 1)
  ▼
Engine.Run → handleCancel:
  │
  │  ① 查订单信息 (撤之前查, 撤之后就没了)
  │     order = book.GetOrder(1) → 知道 side/price/qty
  │
  │  ② 从订单簿删除
  │     book.CancelOrder(1)
  │       从 PriceLevel.Orders 切片中删除
  │       如果档位空了 → 从 OrderedSide 删除
  │       从 orders/locate map 删除
  │
  │  ③ 解冻资金
  │     Buy 单 → ledger.Unfreeze(owner, "USD", price*qty)
  │     Sell 单 → ledger.Unfreeze(owner, "ETH", qty)
  │
  │  ④ 发布盘口变化
  │     bus.Publish(Event{Type:"book_update"})
  │
  │  cmd.Reply ← Result{}
  ▼
浏览器收到 200 OK
```

## 4. 并发模型

```
                  goroutine 数量
Handler A ─┐
Handler B ─┤        N 个 (每个 HTTP 请求一个)
Handler C ─┤
  ...      ─┤
            │
            ▼
         cmds channel (缓冲 64)
            │
            ▼
         Engine.Run    1 个 (唯一碰 Book 的)
            │
            ├──▶ Book      不需要锁 (单 goroutine 独占)
            │
            └──▶ Ledger    需要 Mutex (多个 Engine 共享)
                              │
                 ETH/USD Engine ──┤
                 BTC/USD Engine ──┘

WebSocket A ─┐
WebSocket B ─┤  N 个 (每个连接一个)
             │
             ▼
          EventBus     需要 RWMutex (Publish 读锁, Subscribe 写锁)
```

**为什么 Book 不加锁**: 只有 Engine.Run 一个 goroutine 访问它, 没有并发。
**为什么 Ledger 加 Mutex**: 多币对时多个 Engine 共享一个 Ledger, 并发写。
**为什么 EventBus 加 RWMutex**: Publish (读 subs map) 和 Subscribe (写 subs map) 并发。

## 5. 数据结构关系

```
Book
 ├── bids: OrderedSide (买方)
 │    ├── prices []int64          [95, 98, 100, 102]  有序, O(1) 取最大
 │    └── levels map[int64]*PriceLevel
 │         └── PriceLevel{Price:100, Orders:[orderID1, orderID2]}  FIFO
 │
 ├── asks: OrderedSide (卖方)
 │    ├── prices []int64          [103, 105, 110]     有序, O(1) 取最小
 │    └── levels map[int64]*PriceLevel
 │
 ├── orders map[int64]Order       所有订单, 按 ID 查 O(1)
 └── locate map[int64]int64       orderID → price, 撤单时 O(1) 定位

Ledger
 └── accounts map[int64]*Account  ownerID → 账户
      └── Account
           ├── Available map[string]int64   {"USD": 9000, "ETH": 10}
           └── Frozen    map[string]int64   {"USD": 1000}

EventBus
 └── subs map[int]*chan Event     subscriberID → 带缓冲 channel
```

## 6. 关键设计决策

| 决策 | 为什么 | 面试怎么答 |
|------|--------|-----------|
| int64 不用 float64 | 0.1+0.2!=0.3, 破坏 map key 和成交判等 | "金融计算用定点数, 避免浮点精度问题" |
| channel actor 不用 mutex (Engine) | 设计层面消除 race, 不是锁层面修补 | "单写 goroutine 独占状态, 通过 channel 通信" |
| mutex 不用 channel (Ledger) | 操作很短(改两个数字), 锁更直接 | "短临界区用锁, 长任务排队用 channel" |
| 冻结在 Submit 之前 | 防止同一笔钱下两次单 | "先冻后挂, TOCTOU 问题必须原子处理" |
| EventBus 满了就丢 | 行情可丢(下一帧覆盖), 不能让发布者阻塞 | "推送行情 vs 推送指令, 丢弃策略不同" |
| 深拷贝快照 | handler goroutine 读快照不 race | "读写分离, 快照是不可变值" |
| 一币对一 Engine | 币对间零锁, 天然并行 | "按 key 分片, 水平扩展的最小单元" |
| Go Mutex 不可重入 | 私有方法不加锁, 调用者保证 | "跟 Java synchronized 不同, 同 goroutine 锁两次会死锁" |
| 市价单不挂簿 | 无流动性应显式暴露 | "市价单剩余丢弃, 避免挂成无价格的哨兵单" |
| 撤单前先 GetOrder | CancelOrder 会删数据, 之后查不到解冻信息 | "先读后删, 顺序不能反" |
