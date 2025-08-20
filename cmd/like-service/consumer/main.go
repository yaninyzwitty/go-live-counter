package main

import (
	"context"
	"log"
	"log/slog"
	"os"

	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/joho/godotenv"
)

func main() {
	log.Println("Pulsar Consumer")
	if err := godotenv.Load(); err != nil {
		slog.Warn("failed to load .env", "error", err)
	}

	tokenStr := os.Getenv("PULSAR_TOKEN")
	if tokenStr == "" {
		slog.Error("missing token string")
		os.Exit(1)
	}

	uri := "pulsar+ssl://pulsar-gcp-europewest1.streaming.datastax.com:6651"
	topicName := "persistent://witty-tenant/default/likes"
	subscriptionName := "test_sub"

	token := pulsar.NewAuthenticationToken(tokenStr)

	// Pulsar client
	client, err := pulsar.NewClient(pulsar.ClientOptions{
		URL:            uri,
		Authentication: token,
	})

	if err != nil {
		log.Fatal(err)
	}

	defer client.Close()

	consumer, err := client.Subscribe(pulsar.ConsumerOptions{
		Topic:                       topicName,
		SubscriptionName:            subscriptionName,
		SubscriptionInitialPosition: pulsar.SubscriptionPositionEarliest,
	})

	if err != nil {
		log.Fatal(err)
	}

	defer consumer.Close()

	ctx := context.Background()

	// infinite loop to receive messages
	for {
		msg, err := consumer.Receive(ctx)
		if err != nil {
			log.Fatal(err)
		} else {
			log.Printf("Received message : %s", string(msg.Payload()))
		}

		consumer.Ack(msg)
	}

}
