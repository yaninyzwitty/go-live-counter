CREATE TABLE IF NOT EXISTS outbox (
    id UUID PRIMARY KEY, -- key stuff consitent by generating with gen_random_uuid in queries
    event_type STRING NOT NULL,
    payload JSONB NOT NULL, -- store as JSONB -> you can use with json/-iter, protojson
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    published BOOLEAN NOT NULL DEFAULT false,
    processed_at TIMESTAMPTZ NULL
);


-- create index on published field, fast retrieval of unprocessed events

CREATE INDEX idx_outbox_unprocessed ON outbox (published, created_at) WHERE published = false;
