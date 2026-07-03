package matching

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
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

// SetupRouter 创建 Gin 路由, 所有 handler 都通过 Engine 的方法访问簿。
// Handler 是薄的、无状态的: 解析 JSON → 调 Engine 方法 → 返回结果。
//
// 你来实现三个 handler。
func SetupRouter(e *Engine) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()

	// POST /orders — 下单
	r.POST("/orders", func(c *gin.Context) {
		var req PlaceOrderRequest
		c.ShouldBindJSON(&req)
		order := Order{
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
	return r
}

// parseSide 把字符串转成 Side 枚举。
func parseSide(s string) Side {
	if s == "sell" {
		return Sell
	}
	return Buy
}

// parseOrderType 把字符串转成 OrderType 枚举。
func parseOrderType(s string) OrderType {
	if s == "market" {
		return Market
	}
	return Limit
}

// parseID 从 URL 参数解析 int64 ID。
func parseID(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
