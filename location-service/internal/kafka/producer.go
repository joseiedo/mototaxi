package kafka

import (
	"context"
	"time"

	"github.com/twmb/franz-go/pkg/kadm"
	"github.com/twmb/franz-go/pkg/kgo"
)

// Producer is the Kafka publish interface. Implemented by franz-go wrapper; mockable in tests.
type Producer interface {
	Publish(ctx context.Context, key string, value []byte) error
	Ping(ctx context.Context) error
	Close()
}

type franzProducer struct {
	cl    *kgo.Client
	topic string
}

// NewProducer creates a new franz-go Kafka producer.
// addr is the broker address (e.g., "redpanda:9092"), topic is the target topic name.
func NewProducer(addr, topic string) (Producer, error) {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers(addr),
		kgo.RequiredAcks(kgo.LeaderAck()),
		kgo.DisableIdempotentWrite(),
		kgo.RecordDeliveryTimeout(5*time.Second),
	)
	if err != nil {
		return nil, err
	}
	return &franzProducer{cl: cl, topic: topic}, nil
}

// Publish sends a record to the Kafka topic synchronously, keyed by key.
// Returns error if ProduceSync fails — caller maps to HTTP 503.
func (p *franzProducer) Publish(ctx context.Context, key string, value []byte) error {
	record := &kgo.Record{
		Topic: p.topic,
		Key:   []byte(key),
		Value: value,
	}
	return p.cl.ProduceSync(ctx, record).FirstErr()
}

// Ping verifies Kafka broker connectivity using kadm.ListBrokers.
// Returns error if broker is unreachable.
func (p *franzProducer) Ping(ctx context.Context) error {
	adm := kadm.NewClient(p.cl)
	_, err := adm.ListBrokers(ctx)
	return err
}

// Close shuts down the underlying kgo.Client.
func (p *franzProducer) Close() {
	p.cl.Close()
}
