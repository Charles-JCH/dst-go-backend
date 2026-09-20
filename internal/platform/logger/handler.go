package logger

import (
	"context"
	"log/slog"
)

type TraceHandler struct {
	slog.Handler
}

// NewTraceHandler 创建包装后的 TraceHandler
func NewTraceHandler(handler slog.Handler) slog.Handler {
	return &TraceHandler{Handler: handler}
}

// Handle 拦截日志记录并追加 traceId
func (h *TraceHandler) Handle(ctx context.Context, r slog.Record) error {
	if traceId := GetTraceId(ctx); traceId != "" {
		r.AddAttrs(slog.String("traceId", traceId))
	}
	return h.Handler.Handle(ctx, r)
}

// WithAttrs 保证链式调用时包装不丢失
func (h *TraceHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithAttrs(attrs)}
}

// WithGroup 保证分组链式调用时包装不丢失
func (h *TraceHandler) WithGroup(name string) slog.Handler {
	return &TraceHandler{Handler: h.Handler.WithGroup(name)}
}
