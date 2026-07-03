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
	// 1. 创建引擎
	engine := matching.NewEngine()

	// 2. 用一个可取消的 context 控制引擎生命周期
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 3. 启动引擎 goroutine
	done := make(chan struct{})
	go func() {
		engine.Run(ctx)
		close(done) // Run 返回后通知主线程
	}()

	// 4. 设置 Gin 路由
	router := matching.SetupRouter(engine)

	// 5. 创建 HTTP 服务器
	srv := &http.Server{
		Addr:    ":8080",
		Handler: router,
	}

	// 6. 在 goroutine 中启动 HTTP 服务
	go func() {
		fmt.Println("交易所启动: http://localhost:8080")
		fmt.Println("  POST   /orders      下单")
		fmt.Println("  DELETE /orders/:id   撤单")
		fmt.Println("  GET    /book         查看盘口")
		fmt.Println("按 Ctrl+C 关停...")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			fmt.Fprintf(os.Stderr, "HTTP 服务异常: %v\n", err)
		}
	}()

	// 7. 等待中断信号 (Ctrl+C)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	fmt.Println("\n收到关停信号, 开始优雅关停...")

	// 8. 先关 HTTP (停止接受新请求, 等待已有请求完成)
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer shutdownCancel()
	srv.Shutdown(shutdownCtx)

	// 9. 再关引擎 (取消 context → Run 返回)
	cancel()
	<-done // 等 Run 真正退出

	fmt.Println("交易所已关停")
}
