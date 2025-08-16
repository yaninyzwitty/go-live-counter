-- name: FindAllUsers :many
SELECT * FROM users;

-- name: InsertItem :one
INSERT INTO users (id, name, email)
VALUES (gen_random_uuid(), $1, $2)
RETURNING *;

-- name: FindItemByID :one
SELECT * FROM users WHERE id = $1;