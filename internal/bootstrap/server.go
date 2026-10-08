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
		slog.Info("http 监听启动中", "addr", addr, "mode", cfg.Mode)
		serverErr <- srv.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return fmt.Errorf("http 监听失败: %w", err)
	case sig := <-quit:
		slog.Info("http 停机开始", "signal", sig.String())
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("http 优雅停机失败，强制关闭连接", "error", err)
		if closeErr := srv.Close(); closeErr != nil {
			return errors.Join(fmt.Errorf("http 优雅停机失败: %w", err), fmt.Errorf("http 强制关闭失败: %w", closeErr))
		}
		return fmt.Errorf("http 优雅停机失败: %w", err)
	}

	if err := <-serverErr; err != nil && !errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http 监听失败: %w", err)
	}

	slog.Info("http 停机完成")
	return nil
}
