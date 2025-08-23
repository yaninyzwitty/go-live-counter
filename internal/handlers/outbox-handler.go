package handlers

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"connectrpc.com/connect"
	"github.com/apache/pulsar-client-go/pulsar"
	"github.com/google/uuid"
	likev1 "github.com/yaninyzwitty/go-live-counter/gen/like/v1"
	outboxv1 "github.com/yaninyzwitty/go-live-counter/gen/outbox/v1"
	"github.com/yaninyzwitty/go-live-counter/gen/outbox/v1/outboxv1connect"
	"github.com/yaninyzwitty/go-live-counter/internal/repository"
	"google.golang.org/protobuf/encoding/protojson"
)

// OutboxStoreServiceHandler implements the OutboxService API.

type OutboxStoreServiceHandler struct {
	outboxv1connect.UnimplementedProcessorServiceHandler
	Queries  repository.Queries
	Producer pulsar.Producer
}

// create new instance of outbox handler
func NewOutbox(queries *repository.Queries, producer pulsar.Producer) *OutboxStoreServiceHandler {
	return &OutboxStoreServiceHandler{
		Queries:  *queries,
		Producer: producer,
	}
}

// eventUnmarshalers maps event types to payload unmarshaling logic.
var eventUnmarshalers = map[string]func([]byte) error{
	"USER_LIKE_EVENT_TYPE": func(payload []byte) error {
		var like likev1.Like
		return protojson.Unmarshal(payload, &like)
	},
	"USER_DISLIKE_EVENT_TYPE": func(payload []byte) error {
		var dislike likev1.DisLike
		return protojson.Unmarshal(payload, &dislike)
	},
}

func (h *OutboxStoreServiceHandler) ProcessOutboxMessage(
	ctx context.Context,
	req *connect.Request[outboxv1.ProcessOutboxMessageRequest],
) (*connect.Response[outboxv1.ProcessOutboxMessageResponse], error) {

	// allow only unprocessed messages
	if req.Msg.Published {
		return nil, connect.NewError(connect.CodeInvalidArgument,
			errors.New("published messages cannot be processed again"))
	}

	events, err := h.GetOutboxMessages(ctx)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal,
			fmt.Errorf("failed to fetch outbox messages: %w", err))
	}

	var processedCount int32
	for _, event := range events {
		slog.Info("sending event", "id", event.ID)

		// routing key: fallback to event ID
		routingKey := event.ID.String()

		// Lookup handler for event type
		if unmarshalFn, ok := eventUnmarshalers[event.EventType]; ok {
			if err := unmarshalFn(event.Payload); err != nil {
				return nil, connect.NewError(connect.CodeInternal,
					fmt.Errorf("failed to unmarshal payload for %s: %w", event.EventType, err))
			}
		} else {
			slog.Warn("unknown event type, skipping", "eventType", event.EventType)
			continue
		}

		// Build message
		msg := &pulsar.ProducerMessage{
			Key:     routingKey,
			Payload: event.Payload,
			Properties: map[string]string{
				"event_type": event.EventType,
			},
		}

		// Async publish to Pulsar
		h.Producer.SendAsync(ctx, msg, func(mi pulsar.MessageID, pm *pulsar.ProducerMessage, err error) {
			if err != nil {
				slog.Error("failed to publish event", "id", event.ID, "error", err)
				return
			}

			slog.Info("successfully published event",
				"id", event.ID,
				"messageID", mi)

			// mark as published in a safe goroutine
			go func(eventID uuid.UUID) {
				cctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				if err := h.Queries.PublishProcessedEvents(cctx, []uuid.UUID{eventID}); err != nil {
					slog.Error("failed to mark event as published", "id", eventID, "error", err)
					return
				}
				slog.Info("event marked as published", "id", eventID)
			}(event.ID)
		})

		processedCount++
	}

	resp := connect.NewResponse(&outboxv1.ProcessOutboxMessageResponse{
		ProcessedCount: processedCount,
	})
	return resp, nil
}

func (h *OutboxStoreServiceHandler) GetOutboxMessages(ctx context.Context) ([]repository.Outbox, error) {
	events, err := h.Queries.FindAndLockUnpublishedEvents(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get unpublished events: %w", err)

	}
	return events, nil

}
