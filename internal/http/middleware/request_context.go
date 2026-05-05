package middleware

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"

	"github.com/Incipe-win/OpenStory/internal/observability"
)

const (
	HeaderRequestID = "X-Request-ID"
	HeaderTraceID   = "X-Trace-ID"
)

func RequestContext(serviceName string) gin.HandlerFunc {
	tracer := otel.Tracer(serviceName + "/api")
	propagator := otel.GetTextMapPropagator()
	if propagator == nil {
		propagator = propagation.TraceContext{}
	}
	return func(c *gin.Context) {
		requestID := c.GetHeader(HeaderRequestID)
		if requestID == "" {
			requestID = uuid.NewString()
		}

		parentCtx := propagator.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))
		ctx, span := tracer.Start(parentCtx, c.Request.Method+" "+c.FullPath())
		defer span.End()

		traceID := observability.TraceIDFromContext(ctx)
		carrier := propagation.MapCarrier{}
		propagator.Inject(ctx, carrier)
		ctx = observability.ContextWithIDs(ctx, requestID, traceID)
		ctx = observability.ContextWithTraceParent(ctx, carrier.Get("traceparent"))
		c.Request = c.Request.WithContext(ctx)
		c.Set("request_id", requestID)
		c.Set("trace_id", traceID)
		c.Writer.Header().Set(HeaderRequestID, requestID)
		if traceID != "" {
			c.Writer.Header().Set(HeaderTraceID, traceID)
		}

		started := time.Now()
		c.Next()

		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		status := c.Writer.Status()
		span.SetName(c.Request.Method + " " + route)
		span.SetAttributes(
			attribute.String("http.method", c.Request.Method),
			attribute.String("http.route", route),
			attribute.Int("http.status_code", status),
			attribute.String("request.id", requestID),
		)
		if status >= 500 {
			span.SetStatus(codes.Error, "server error")
		}
		observability.ObserveAPIRequest(c.Request.Method, route, status, time.Since(started))
	}
}
