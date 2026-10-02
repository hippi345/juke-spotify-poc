package events

import (
	"context"
	"encoding/json"
	"testing"
)

func TestTopicNames(t *testing.T) {
	t.Parallel()
	if TopicVote != "jukespotify.vote" || TopicPayment != "jukespotify.payment" || TopicSession != "jukespotify.session" {
		t.Fatalf("unexpected topic constants")
	}
}

func TestNopPublisher(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	var pub Publisher = nopPublisher{}

	if err := pub.PublishVote(ctx, VotePayload{SessionID: 1}); err != nil {
		t.Fatalf("PublishVote: %v", err)
	}
	if err := pub.PublishPayment(ctx, PaymentPayload{PaidSkipID: 1}); err != nil {
		t.Fatalf("PublishPayment: %v", err)
	}
	if err := pub.PublishSession(ctx, SessionPayload{Action: "started", SessionID: 2}); err != nil {
		t.Fatalf("PublishSession: %v", err)
	}
}

func TestInitKafkaEmptyBrokers(t *testing.T) {
	prev := Default
	t.Cleanup(func() { Default = prev })

	if err := InitKafka(""); err != nil {
		t.Fatalf("InitKafka empty: %v", err)
	}
}

func TestSessionPayloadJSON(t *testing.T) {
	t.Parallel()
	vid := uint(9)
	raw, err := json.Marshal(SessionPayload{Action: "started", SessionID: 99, VenueID: &vid})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back SessionPayload
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Action != "started" || back.SessionID != 99 || back.VenueID == nil || *back.VenueID != 9 {
		t.Fatalf("roundtrip: %+v", back)
	}
}
