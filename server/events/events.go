package events

import (
	"context"
	"encoding/json"
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

const (
	TopicVote    = "jukespotify.vote"
	TopicPayment = "jukespotify.payment"
	TopicSession = "jukespotify.session"
)

// VotePayload is published when a patron successfully votes.
type VotePayload struct {
	SessionID uint   `json:"session_id"`
	PatronID  uint   `json:"patron_id"`
	TrackID   string `json:"track_id"`
}

// PaymentPayload is published when a paid skip is marked paid.
type PaymentPayload struct {
	PaidSkipID      uint `json:"paid_skip_id"`
	VotingSessionID uint `json:"voting_session_id"`
}

// SessionPayload is published when a voting session starts or ends.
type SessionPayload struct {
	Action    string `json:"action"` // started | ended
	SessionID uint   `json:"session_id"`
	VenueID   *uint  `json:"venue_id,omitempty"`
}

// Publisher sends domain events to Kafka (no-op when brokers unset).
type Publisher interface {
	PublishVote(ctx context.Context, p VotePayload) error
	PublishPayment(ctx context.Context, p PaymentPayload) error
	PublishSession(ctx context.Context, p SessionPayload) error
}

type nopPublisher struct{}

func (nopPublisher) PublishVote(context.Context, VotePayload) error    { return nil }
func (nopPublisher) PublishPayment(context.Context, PaymentPayload) error { return nil }
func (nopPublisher) PublishSession(context.Context, SessionPayload) error { return nil }

// Default is a no-op until InitKafka succeeds.
var Default Publisher = nopPublisher{}

type kafkaPublisher struct {
	writers map[string]*kafka.Writer
}

func (k *kafkaPublisher) write(ctx context.Context, topic string, v interface{}) error {
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	headers := make([]kafka.Header, 0, 4)
	injectTraceContext(ctx, &headers)
	w := k.writers[topic]
	return w.WriteMessages(ctx, kafka.Message{
		Headers: headers,
		Key:     []byte(topic),
		Value:   raw,
		Time:    time.Now(),
	})
}

func (k *kafkaPublisher) PublishVote(ctx context.Context, p VotePayload) error {
	return k.write(ctx, TopicVote, p)
}
func (k *kafkaPublisher) PublishPayment(ctx context.Context, p PaymentPayload) error {
	return k.write(ctx, TopicPayment, p)
}
func (k *kafkaPublisher) PublishSession(ctx context.Context, p SessionPayload) error {
	return k.write(ctx, TopicSession, p)
}

// InitKafka configures the Kafka publisher. brokers is a comma-separated list.
func InitKafka(brokers string) error {
	brokers = strings.TrimSpace(brokers)
	if brokers == "" {
		return nil
	}
	addrs := strings.Split(brokers, ",")
	pub := &kafkaPublisher{writers: map[string]*kafka.Writer{}}
	for _, topic := range []string{TopicVote, TopicPayment, TopicSession} {
		pub.writers[topic] = &kafka.Writer{
			Addr:         kafka.TCP(addrs...),
			Topic:        topic,
			Balancer:     &kafka.LeastBytes{},
			RequiredAcks: kafka.RequireOne,
			Async:        false,
		}
	}
	Default = pub
	return nil
}

// Handler processes consumed Kafka messages (used by API replicas).
type Handler func(ctx context.Context, topic string, value []byte, headers []kafka.Header) error

// RunConsumer starts a consumer group loop until ctx is cancelled.
func RunConsumer(ctx context.Context, brokers string, groupID string, topics []string, h Handler) {
	brokers = strings.TrimSpace(brokers)
	if brokers == "" || len(topics) == 0 {
		return
	}
	addrs := strings.Split(brokers, ",")
	for _, topic := range topics {
		go func(topic string) {
			r := kafka.NewReader(kafka.ReaderConfig{
				Brokers:  addrs,
				GroupID:  groupID,
				Topic:    topic,
				MinBytes: 1,
				MaxBytes: 10e6,
			})
			defer func() { _ = r.Close() }()
			for {
				msg, err := r.ReadMessage(ctx)
				if err != nil {
					if ctx.Err() != nil {
						return
					}
					log.Printf("kafka: read %s: %v", topic, err)
					time.Sleep(time.Second)
					continue
				}
				msgCtx := extractTraceContext(ctx, msg.Headers)
				if err := h(msgCtx, topic, msg.Value, msg.Headers); err != nil {
					log.Printf("kafka: handle %s: %v", topic, err)
				}
			}
		}(topic)
	}
}
