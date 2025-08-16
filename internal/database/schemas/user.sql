
CREATE TABLE IF NOT EXISTS users (
  id          UUID PRIMARY KEY,
  name        TEXT NOT NULL,
  email       TEXT UNIQUE,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);