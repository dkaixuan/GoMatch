package engine

import (
	"encoding/json"
	"net/http"
	"strconv"

	"exchange/eventbus"
	"exchange/matching"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
)

// PlaceOrderRequest 是 POST /orders 的请求体。
type PlaceOrderRequest struct {
	ID      int64  `json:"id"`
	OwnerID int64  `json:"owner_id"`
	Side    string `json:"side"` // "buy" 或 "sell"
	Type    string `json:"type"` // "limit" 或 "market"
	Price   int64  `json:"price"`
	Qty     int64  `json:"qty"`
}

// SetupRouter 创建 Gin 路由。
func SetupRouter(e *Engine) *gin.Engine {
	return SetupRouterWithBus(e, nil)
}

var wsUpgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true },
}

// SetupRouterWithBus 创建带 WebSocket 推送的 Gin 路由。
func SetupRouterWithBus(e *Engine, bus *eventbus.EventBus) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// POST /orders — 下单
	r.POST("/orders", func(c *gin.Context) {
		var req PlaceOrderRequest
		c.ShouldBindJSON(&req)
		order := matching.Order{
			ID:      req.ID,
			OwnerID: req.OwnerID,
			Side:    parseSide(req.Side),
			Type:    parseOrderType(req.Type),
			Price:   req.Price,
			Qty:     req.Qty,
		}
		trades, err := e.Place(c.Request.Context(), order)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, gin.H{"trades": trades})
	})

	// GET /book — 获取盘口快照
	r.GET("/book", func(c *gin.Context) {
		result, err := e.GetSnapshot(c.Request.Context())
		if err != nil {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.JSON(http.StatusOK, result)
	})

	// DELETE /orders/:id — 撤单
	r.DELETE("/orders/:id", func(c *gin.Context) {
		id, err := parseID(c.Param("id"))
		if err != nil {
			c.Status(http.StatusBadRequest)
			return
		}
		err = e.Cancel(c.Request.Context(), id)
		if err != nil {
			c.Status(http.StatusNotFound)
			return
		}
		c.Status(http.StatusOK)
	})

	// GET /ws — WebSocket 实时推送
	if bus != nil {
		r.GET("/ws", func(c *gin.Context) {
			conn, err := wsUpgrader.Upgrade(c.Writer, c.Request, nil)
			if err != nil {
				return
			}
			defer conn.Close()

			id, ch := bus.Subscribe(64)
			defer bus.Unsubscribe(id)

			// 读 goroutine: 检测客户端断开
			done := make(chan struct{})
			go func() {
				defer close(done)
				for {
					if _, _, err := conn.ReadMessage(); err != nil {
						return
					}
				}
			}()

			// 写循环: 把事件推给客户端
			for {
				select {
				case event, ok := <-ch:
					if !ok {
						return
					}
					data, _ := json.Marshal(event)
					if err := conn.WriteMessage(websocket.TextMessage, data); err != nil {
						return
					}
				case <-done:
					return
				}
			}
		})
	}

	return r
}

// parseSide 把字符串转成 Side 枚举。
func parseSide(s string) matching.Side {
	if s == "sell" {
		return matching.Sell
	}
	return matching.Buy
}

// parseOrderType 把字符串转成 OrderType 枚举。
func parseOrderType(s string) matching.OrderType {
	if s == "market" {
		return matching.Market
	}
	return matching.Limit
}

// parseID 从 URL 参数解析 int64 ID。
func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
