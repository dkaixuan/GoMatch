# GoMatch

Go 实现的纯内存限价撮合交易所。

## 快速开始

```bash
go run main.go
```

```bash
# 挂卖单
curl -X POST localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"id":1, "side":"sell", "type":"limit", "price":100, "qty":10}'

# 提交买单（成交）
curl -X POST localhost:8080/orders \
  -H "Content-Type: application/json" \
  -d '{"id":2, "side":"buy", "type":"limit", "price":100, "qty":10}'

# 查看盘口
curl localhost:8080/book

# 撤单
curl -X DELETE localhost:8080/orders/1
```

## API

| 方法 | 路径 | 说明 | 状态码 |
|------|------|------|--------|
| POST | `/orders` | 下单 | 201 |
| DELETE | `/orders/:id` | 撤单 | 200 / 404 |
| GET | `/book` | 盘口快照 | 200 |

### 请求体示例

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

## 架构

```
Gin Handler → Command channel → Engine (单 goroutine) → Book
```

只有 Engine 的 Run goroutine 访问订单簿，HTTP handler 通过 channel 通信，不加锁。

## 项目结构

```
├── main.go              # 入口，启动 + 关停
├── matching/
│   ├── order.go         # Order, Trade, Side 等类型
│   ├── pricelevel.go    # 价格档位 FIFO 队列
│   ├── book.go          # 订单簿，撮合逻辑
│   ├── engine.go        # 并发引擎
│   ├── handler.go       # Gin HTTP handler
│   └── *_test.go
└── docs/
    └── 00-syllabus.md
```

## 测试

```bash
go test ./matching/ -v
```

## License

MIT
