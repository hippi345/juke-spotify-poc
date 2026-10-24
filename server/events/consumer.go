package events

import (
	"log"
	"strings"
	"time"

	"github.com/segmentio/kafka-go"
)

const kafkaReadyTimeout = 2 * time.Minute

// waitForKafkaBroker blocks until a TCP connection to the first broker succeeds or timeout.
func waitForKafkaBroker(brokers string) bool {
	brokers = strings.TrimSpace(brokers)
	if brokers == "" {
		return false
	}
	addrs := strings.Split(brokers, ",")
	deadline := time.Now().Add(kafkaReadyTimeout)
	for attempt := 1; time.Now().Before(deadline); attempt++ {
		for _, addr := range addrs {
			addr = strings.TrimSpace(addr)
			if addr == "" {
				continue
			}
			conn, err := kafka.Dial("tcp", addr)
			if err == nil {
				_ = conn.Close()
				if attempt > 1 {
					log.Printf("kafka: broker %s ready after %d attempt(s)", addr, attempt)
				}
				return true
			}
		}
		delay := time.Duration(attempt) * 2 * time.Second
		if delay > 15*time.Second {
			delay = 15 * time.Second
		}
		log.Printf("kafka: waiting for broker (%v), retry in %v", addrs, delay)
		time.Sleep(delay)
	}
	log.Printf("kafka: broker not reachable within %v", kafkaReadyTimeout)
	return false
}

func ensureKafkaTopics(brokers string, topics []string) {
	brokers = strings.TrimSpace(brokers)
	if brokers == "" || len(topics) == 0 {
		return
	}
	addr := strings.TrimSpace(strings.Split(brokers, ",")[0])
	conn, err := kafka.Dial("tcp", addr)
	if err != nil {
		log.Printf("kafka: ensure topics dial failed: %v", err)
		return
	}
	defer func() { _ = conn.Close() }()
	configs := make([]kafka.TopicConfig, 0, len(topics))
	for _, t := range topics {
		configs = append(configs, kafka.TopicConfig{
			Topic:             t,
			NumPartitions:     1,
			ReplicationFactor: 1,
		})
	}
	if err := conn.CreateTopics(configs...); err != nil {
		log.Printf("kafka: create topics: %v", err)
	}
}
