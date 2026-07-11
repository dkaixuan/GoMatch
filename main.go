package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"exchange/matching"
)

func main() {
	// 1. 创建共享组件
	ledger := matching.NewLedger()
	bus := matching.NewEventBus()

	// 2. 创建引擎(ETH/USD)
	engine := matching.NewEngineWithLedger(ledger, "ETH", "USD")
	engine.SetEventBus(bus)

	// 3. 给演示用户入金
	ledger.Deposit(1, "USD", 1000000)
	ledger.Deposit(1, "ETH", 1000)
	ledger.Deposit(2, "USD", 1000000)
	ledger.Deposit(2, "ETH", 1000)

	// 4. 用一个可取消的 context 控制引擎生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	go func() {
		engine.Run(ctx)
		close(done)
	}()

	// 5. 设置路由(带 WebSocket)
	router := matching.SetupRouterWithBus(engine, bus)
	router.StaticFile("/", "./static/index.html")
	router.Static("/static", "./static")

	// 6. 创建 HTTP 服务器
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// 7. 启动
	go func() {
		fmt.Println("GoMatch Exchange 启动: http://localhost:8080/static/index.html")
		fmt.Println("  POST   /orders      下单")
		fmt.Println("  DELETE /orders/:id   撤单")
		fmt.Println("  GET    /book         盘口快照")
		fmt.Println("  GET    /ws           WebSocket 实时推送")
		fmt.Println()
		fmt.Println("演示用户: ID=1 和 ID=2, 各有 1000000 USD + 1000 ETH")
		fmt.Println("按 Ctrl+C 关停...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP 服务异常: %v\n", err)
		}
	}()

	// 8. 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n收到关停信号...")
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)
	cancel()
	<-done
	fmt.Println("已关停")
}
