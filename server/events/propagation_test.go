package events

import (
	"context"
	"testing"

	"github.com/segmentio/kafka-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func TestKafkaTracePropagationRoundTrip(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	t.Cleanup(func() { _ = tp.Shutdown(context.Background()) })
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
	))

	ctx, span := tp.Tracer("test").Start(context.Background(), "produce")
	defer span.End()

	var headers []kafka.Header
	injectTraceContext(ctx, &headers)
	if len(headers) == 0 {
		t.Fatal("expected trace headers after inject")
	}

	sc := span.SpanContext()
	extracted := extractTraceContext(context.Background(), headers)
	_, consumeSpan := tp.Tracer("test").Start(extracted, "consume")
	defer consumeSpan.End()
	esc := consumeSpan.SpanContext()
	if !esc.IsValid() {
		t.Fatal("expected valid span context after extract")
	}
	if sc.TraceID() != esc.TraceID() {
		t.Fatalf("trace id mismatch: %s vs %s", sc.TraceID(), esc.TraceID())
	}
}

func TestKafkaHeadersCarrier(t *testing.T) {
	t.Parallel()
	h := []kafka.Header{{Key: "traceparent", Value: []byte("00-abc-def-01")}}
	c := kafkaHeadersCarrier{headers: &h}
	if c.Get("traceparent") != "00-abc-def-01" {
		t.Fatalf("Get: %q", c.Get("traceparent"))
	}
	c.Set("baggage", "k=v")
	if len(*c.headers) != 2 {
		t.Fatalf("Set: len=%d", len(*c.headers))
	}
	keys := c.Keys()
	if len(keys) != 2 {
		t.Fatalf("Keys: %v", keys)
	}
}
