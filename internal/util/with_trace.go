package util

import (
	"context"

	"go.opentelemetry.io/otel/trace"
	"go.uber.org/zap"
)

func WithTrace(ctx context.Context, l *zap.SugaredLogger) *zap.SugaredLogger {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if sc.HasTraceID() {
		l = l.With(zap.String("trace_id", sc.TraceID().String()))
	}
	return l
}
