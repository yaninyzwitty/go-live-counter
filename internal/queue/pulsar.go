package queue

import (
	"context"
	"fmt"
	"time"

	"github.com/apache/pulsar-client-go/pulsar"
)

type PulsarWrapper struct {
	Client pulsar.Client
}

type PulsarConfig struct {
	ServiceURL string
	TokenStr   string
}

// NewPulsarWrapper initializes the Pulsar client
func NewPulsarWrapper(cfg PulsarConfig) (*PulsarWrapper, error) {
	token := pulsar.NewAuthenticationToken(cfg.TokenStr)

	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:            cfg.ServiceURL,
		Authentication: token,
		// You can tune these if needed:
		// OperationTimeout:  30 * time.Second,
		// ConnectionTimeout: 30 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create pulsar client: %w", err)
	}

	return &PulsarWrapper{Client: client}, nil
}

// CreateProducer wraps pulsar producer creation
func (pw *PulsarWrapper) CreateProducer(topic string) (pulsar.Producer, error) {
	return pw.Client.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})
}

// CreateConsumer wraps pulsar consumer creation
func (pw *PulsarWrapper) CreateConsumer(topic, subscription string, subType pulsar.SubscriptionType) (pulsar.Consumer, error) {
	return pw.Client.Subscribe(pulsar.ConsumerOptions{
		Topic:                       topic,
		SubscriptionName:            subscription,
		Type:                        subType, // pulsar.Shared, Exclusive, Failover, KeyShared
		SubscriptionInitialPosition: pulsar.SubscriptionPositionEarliest,
	})
}

// SendMessage sends a message to a producer (async under the hood)
func (pw *PulsarWrapper) SendMessageAsync(ctx context.Context, producer pulsar.Producer, payload []byte, onComplete func(pulsar.MessageID, error)) {
	producer.SendAsync(ctx, &pulsar.ProducerMessage{}, func(mi pulsar.MessageID, pm *pulsar.ProducerMessage, err error) {
		if onComplete != nil {
			onComplete(mi, err)
		}
	})
}

// ReceiveMessage receives one message with timeout
func (pw *PulsarWrapper) ReceiveMessage(ctx context.Context, consumer pulsar.Consumer, timeout time.Duration) (pulsar.Message, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	msg, err := consumer.Receive(ctx)
	if err != nil {
		return nil, err
	}
	return msg, nil
}

// AckMessage acknowledges a consumed message
func (pw *PulsarWrapper) AckMessage(consumer pulsar.Consumer, msg pulsar.Message) {
	consumer.Ack(msg)
}

// Close closes the Pulsar client
func (pw *PulsarWrapper) Close() {
	pw.Client.Close()
}
