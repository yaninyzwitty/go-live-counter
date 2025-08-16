-- name: FindAllUsers :many
SELECT * FROM users;

-- name: InsertItem :one
INSERT INTO users (id, name, email)
VALUES (uuid_generate_v4(), $1, $2)
RETURNING *;

-- name: FindItemByID :one
SELECT * FROM users WHERE id = $1;