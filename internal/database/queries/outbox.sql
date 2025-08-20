-- name: InsertPayloadEvent :one
INSERT INTO outbox (id, event_type, payload, published)
VALUES (
    gen_random_uuid(),  -- CockroachDB-safe UUID
    $1,                 -- event_type
    $2,                 -- payload (JSONB)
    false               -- always start unpublished
)
RETURNING *;


-- name: FindAndLockUnpublishedEvents :many
SELECT * FROM outbox
WHERE published = false
ORDER BY created_at
LIMIT 10
FOR UPDATE SKIP LOCKED;

-- name: PublishProcessedEvent :one
UPDATE outbox
SET published = true, processed_at = now()
WHERE id = $1
RETURNING *;

-- name: PublishProcessedEvents :exec
UPDATE outbox
SET published = true, processed_at = now()
WHERE id = ANY($1::uuid[]);
