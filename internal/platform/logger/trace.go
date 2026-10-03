package logger

import "context"

type contextKey string

const traceIDKey contextKey = "trace_id"

const TraceHeaderKey = "X-Trace-ID"

// WithTraceID 将 traceID 注入 Context
func WithTraceID(ctx context.Context, traceID string) context.Context {
	return context.WithValue(ctx, traceIDKey, traceID)
}

// GetTraceID 从 context.Context 中获取 traceID
func GetTraceID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceID, ok := ctx.Value(traceIDKey).(string); ok {
		return traceID
	}
	return ""
}
