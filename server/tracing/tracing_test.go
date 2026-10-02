package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

func TestInitEmptyEndpointSetsPropagator(t *testing.T) {
	shutdown, err := Init(context.Background(), "", "juke-api", "test-1")
	if err != nil {
		t.Fatalf("Init: %v", err)
	}
	if shutdown != nil {
		t.Fatal("expected nil shutdown when endpoint empty")
	}
	prop := otel.GetTextMapPropagator()
	if _, ok := prop.(propagation.TextMapPropagator); !ok {
		t.Fatal("expected text map propagator")
	}
	carrier := propagation.MapCarrier{}
	ctx := otel.GetTextMapPropagator().Extract(context.Background(), carrier)
	if ctx == nil {
		t.Fatal("expected non-nil context")
	}
}
