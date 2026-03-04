package main

import (
	"net/http"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"
)

const tracerName = "vericore-api"

var propagator = propagation.NewCompositeTextMapPropagator(
	propagation.TraceContext{},
	propagation.Baggage{},
)

// OTelMiddleware wraps each request in a root span and extracts trace context from headers.
func OTelMiddleware(next http.Handler) http.Handler {
	tracer := otel.Tracer(tracerName)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := propagator.Extract(r.Context(), propagation.HeaderCarrier(r.Header))
		spanName := r.Method + " " + r.URL.Path
		if spanName == " " {
			spanName = r.Method
		}
		ctx, span := tracer.Start(ctx, spanName,
			trace.WithSpanKind(trace.SpanKindServer),
			trace.WithAttributes(
				attribute.String("http.method", r.Method),
				attribute.String("http.url", r.URL.Path),
			),
		)
		defer span.End()
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
