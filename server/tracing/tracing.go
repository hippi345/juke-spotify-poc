package tracing

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.24.0"
)

// Init configures OTLP export to Tempo when endpoint is set. Returns a shutdown func (may be nil).
func Init(ctx context.Context, endpoint, serviceName, instanceID string) (func(context.Context) error, error) {
	endpoint = strings.TrimSpace(endpoint)
	if endpoint == "" {
		otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
			propagation.TraceContext{},
			propagation.Baggage{},
		))
		return nil, nil
	}

	host := strings.TrimPrefix(endpoint, "http://")
	host = strings.TrimPrefix(host, "https://")

	exporter, err := otlptracegrpc.New(ctx,
		otlptracegrpc.WithEndpoint(host),
		otlptracegrpc.WithInsecure(),
	)
	if err != nil {
		return nil, fmt.Errorf("otlp exporter: %w", err)
	}

	if serviceName == "" {
		serviceName = "juke-api"
	}
	attrs := []attribute.KeyValue{
		semconv.ServiceName(serviceName),
	}
	if instanceID != "" {
		attrs = append(attrs, semconv.ServiceInstanceID(instanceID))
	}

	res, err := resource.New(ctx,
		resource.WithAttributes(attrs...),
		resource.WithFromEnv(),
	)
	if err != nil {
		return nil, fmt.Errorf("otel resource: %w", err)
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return func(shutdownCtx context.Context) error {
		ctx, cancel := context.WithTimeout(shutdownCtx, 5*time.Second)
		defer cancel()
		return tp.Shutdown(ctx)
	}, nil
}

// InstrumentationName is the OpenTelemetry tracer name for this API.
const InstrumentationName = "juke-spotify-poc/server"
