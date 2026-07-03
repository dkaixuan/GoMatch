# GoMatch

纯内存限价撮合交易所，Go 语言从零实现，全程 TDD。

## 特性

- **价格时间优先撮合** — 限价单 / 市价单，部分成交，多档扫单
- **单写 Actor 并发模型** — 一个 goroutine 独占订单簿，channel 通信，无锁设计
- **HTTP/JSON API** — Gin 框架，薄 handler，支持下单 / 撤单 / 查看盘口
- **int64 价格** — 拒绝 float64，杜绝 `0.1 + 0.2 != 0.3` 的精度陷阱
- **自成交防护** — 同一用户的买卖单不会互相成交
- **优雅关停** — 信号驱动，先关 HTTP 再停引擎，零 goroutine 泄漏

## 架构

```
HTTP Request
    │
    ▼
┌─────────────────┐
│  Gin Handler    │  解析 JSON → 调 Engine 方法 → 返回结果
└────────┬────────┘
         │ Command channel
         ▼
┌─────────────────┐
│  Engine (Run)   │  单 goroutine，for-select 循环，独占 Book
└────────┬────────┘
         │ 直接调用
         ▼
┌─────────────────┐
│  Book           │  订单簿：AddOrder / CancelOrder / Submit / Snapshot
└─────────────────┘
```

## 快速开始

```bash
# 启动
go run main.go

# 下一笔卖单
curl -X POST localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"id":1, "side":"sell", "type":"limit", "price":100, "qty":10}'

# 下一笔买单（成交）
curl -X POST localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"id":2, "side":"buy", "type":"limit", "price":100, "qty":10}'

# 查看盘口
curl localhost:8080/book

# 撤单
curl -X DELETE localhost:8080/orders/1
```

## API

| 方法 | 路径 | 说明 | 成功状态码 |
|------|------|------|-----------|
| POST | `/orders` | 下单（限价/市价） | 201 |
| DELETE | `/orders/:id` | 撤单 | 200 |
| GET | `/book` | 获取盘口快照 | 200 |

### POST /orders 请求体

```json
{
  "id": 1,
  "owner_id": 100,
  "side": "buy",
  "type": "limit",
  "price": 100,
  "qty": 10
}
```

### POST /orders 响应

```json
{
  "trades": [
    {
      "TakerOrderID": 2,
      "MakerOrderID": 1,
      "Price": 100,
      "Qty": 10
    }
  ]
}
```

## 项目结构

```
GoMatch/
├── main.go                # 入口：启动引擎 + HTTP 服务 + 优雅关停
├── matching/
│   ├── order.go           # 领域类型：Side, OrderType, Order, Trade
│   ├── neworder.go        # 订单校验构造器
│   ├── crosses.go         # 交叉判断：buy >= sell
│   ├── pricelevel.go      # 价格档位 FIFO 队列
│   ├── book.go            # 订单簿：增删改查 + 撮合 + 快照
│   ├── engine.go          # 并发引擎：Command/Result + Run 循环
│   ├── handler.go         # HTTP handler（Gin）
│   └── *_test.go          # 全部测试
└── docs/
    └── 00-syllabus.md     # TDD 教学大纲
```

## 测试

```bash
go test ./matching/ -v
```

共 34 个测试，覆盖：

- 领域类型与校验
- 订单簿 FIFO / 增删改 / 最优价 / 空档清理
- 撮合引擎：全成交 / 部分成交 / 多档扫单 / 市价单 / 自成交防护 / 确定性
- 并发：100 goroutine 交错操作不 panic
- 快照隔离
- HTTP 集成测试
- 优雅关停

## 设计决策

| 决策 | 理由 |
|------|------|
| `int64` 不用 `float64` | 浮点破坏成交相等与 map key |
| 单写 channel 不用 mutex | 设计层面消除 race，不是锁层面 |
| 市价单不挂簿 | 无流动性应响亮暴露，不该悄悄挂成哨兵价 |
| 深拷贝快照 | handler goroutine 零锁读，不 race |
| `context.Context` 控制生命周期 | 取消传播，不 close channel（close 后发会 panic） |

## License

MIT
