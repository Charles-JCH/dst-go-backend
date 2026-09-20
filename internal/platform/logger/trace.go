package logger

import "context"

type contextKey string

const traceIdKey contextKey = "trace_id"

const TraceHeaderKey = "X-Trace-Id"

// WithTraceId 将 traceId 注入 Context
func WithTraceId(ctx context.Context, traceId string) context.Context {
	return context.WithValue(ctx, traceIdKey, traceId)
}

// GetTraceId 从 context.Context 中获取 traceId
func GetTraceId(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if traceId, ok := ctx.Value(traceIdKey).(string); ok {
		return traceId
	}
	return ""
}
