package bootstrap

import (
	"context"
	"errors"
	"fmt"
	"game-panel/internal/config"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func RunHTTPServer(cfg config.ServerConfig, handler *gin.Engine) error {
	addr := fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)

	srv := &http.Server{
		Addr:           addr,
		Handler:        handler,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	serverErr := make(chan error, 1)

	go func() {
		slog.Info("HTTP 服务正在启动监听", "地址", addr, "运行模式", cfg.Mode)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("HTTP 服务监听失败: %w", err)

	case sig := <-quit:
		slog.Info("收到退出信号，开始优雅停机", "信号", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("HTTP 服务优雅停机失败，强制关闭连接", "错误", err)

		if closeErr := srv.Close(); closeErr != nil {
			return errors.Join(fmt.Errorf("HTTP 服务优雅停机失败: %w", err), fmt.Errorf("HTTP 服务强制关闭失败: %w", closeErr))
		}
		return fmt.Errorf("HTTP 服务优雅停机失败: %w", err)
	}

	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("HTTP 服务监听失败: %w", err)
	}

	slog.Info("HTTP 服务已安全停止")
	return nil
}
