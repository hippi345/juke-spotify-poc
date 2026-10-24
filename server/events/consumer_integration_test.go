//go:build integration

package events

import (
	"context"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/segmentio/kafka-go"
)

// Exercises one consumer group subscribed to all jukespotify.* topics (cold-start regression).
// From the host with compose up (published port 29092): KAFKA_BROKERS=host.docker.internal:29092 go test -tags=integration -run TestConsumerAllTopicsSingleGroup ./events/
// On Linux host OS, localhost:29092 reaches the broker but metadata may advertise host.docker.internal; prefer running the test from a container on the compose network.
func TestConsumerAllTopicsSingleGroup_integration(t *testing.T) {
	brokers := os.Getenv("KAFKA_BROKERS")
	if brokers == "" {
		brokers = "host.docker.internal:29092"
	}
	topics := []string{TopicVote, TopicPayment, TopicSession}
	ensureKafkaTopics(brokers, topics)

	groupID := "juke-test-" + time.Now().UTC().Format("20060102T150405") + "-" + uuid.NewString()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var mu sync.Mutex
	seen := make(map[string]bool)
	handler := func(_ context.Context, topic string, _ []byte, _ []kafka.Header) error {
		mu.Lock()
		seen[topic] = true
		mu.Unlock()
		return nil
	}
	RunConsumer(ctx, brokers, groupID, topics, handler)

	pubCtx := context.Background()
	if err := InitKafka(brokers); err != nil {
		t.Fatalf("InitKafka: %v", err)
	}
	if err := Default.PublishSession(pubCtx, SessionPayload{Action: "started", SessionID: 4242}); err != nil {
		t.Fatalf("publish session: %v", err)
	}
	if err := Default.PublishVote(pubCtx, VotePayload{SessionID: 1, PatronID: 1, TrackID: "t"}); err != nil {
		t.Fatalf("publish vote: %v", err)
	}
	if err := Default.PublishPayment(pubCtx, PaymentPayload{PaidSkipID: 1, VotingSessionID: 1}); err != nil {
		t.Fatalf("publish payment: %v", err)
	}

	deadline := time.Now().Add(30 * time.Second)
	for time.Now().Before(deadline) {
		mu.Lock()
		ok := seen[TopicSession] && seen[TopicVote] && seen[TopicPayment]
		mu.Unlock()
		if ok {
			return
		}
		time.Sleep(500 * time.Millisecond)
	}
	mu.Lock()
	got := seen
	mu.Unlock()
	t.Fatalf("timed out waiting for all topics; seen=%v", got)
}
