package observability

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

type contextKey string

const (
	requestIDKey   contextKey = "openstory_request_id"
	traceIDKey     contextKey = "openstory_trace_id"
	traceParentKey contextKey = "openstory_traceparent"
)

func ContextWithRequestID(ctx context.Context, requestID string) context.Context {
	if requestID == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDKey, requestID)
}

func ContextWithTraceID(ctx context.Context, traceID string) context.Context {
	if traceID == "" {
		return ctx
	}
	return context.WithValue(ctx, traceIDKey, traceID)
}

func ContextWithTraceParent(ctx context.Context, traceParent string) context.Context {
	if traceParent == "" {
		return ctx
	}
	return context.WithValue(ctx, traceParentKey, traceParent)
}

func ContextWithIDs(ctx context.Context, requestID, traceID string) context.Context {
	ctx = ContextWithRequestID(ctx, requestID)
	ctx = ContextWithTraceID(ctx, traceID)
	return ctx
}

func ContextWithTraceParentHeader(ctx context.Context, traceParent string) context.Context {
	if traceParent == "" {
		return ctx
	}
	return otel.GetTextMapPropagator().Extract(ctx, propagation.MapCarrier{"traceparent": traceParent})
}

func RequestIDFromContext(ctx context.Context) string {
	value, _ := ctx.Value(requestIDKey).(string)
	return value
}

func TraceIDFromContext(ctx context.Context) string {
	if spanCtx := trace.SpanContextFromContext(ctx); spanCtx.HasTraceID() {
		return spanCtx.TraceID().String()
	}
	value, _ := ctx.Value(traceIDKey).(string)
	return value
}

func TraceParentFromContext(ctx context.Context) string {
	value, _ := ctx.Value(traceParentKey).(string)
	if value != "" {
		return value
	}
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	return carrier.Get("traceparent")
}
