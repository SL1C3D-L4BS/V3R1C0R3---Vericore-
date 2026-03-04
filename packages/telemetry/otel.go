package telemetry

import (
	"context"
	"os"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/exporters/stdout/stdouttrace"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

// InitProvider initializes the global OpenTelemetry TracerProvider.
// If OTEL_EXPORTER_OTLP_ENDPOINT is set, uses OTLP HTTP exporter; otherwise uses stdout for local debugging.
// Returns a shutdown function that must be called (e.g. defer) to flush and stop the provider.
func InitProvider(ctx context.Context) (func(context.Context) error, error) {
	var exp sdktrace.SpanExporter
	var err error

	endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT")
	if endpoint != "" {
		// OTLP HTTP exporter; reads OTEL_EXPORTER_OTLP_* from env when using New(ctx) with no options.
		exp, err = otlptracehttp.New(ctx)
		if err != nil {
			return nil, err
		}
	} else {
		exp, err = stdouttrace.New(stdouttrace.WithPrettyPrint())
		if err != nil {
			return nil, err
		}
	}

	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp))
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}
