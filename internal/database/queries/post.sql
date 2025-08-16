-- name: FindAllPosts :many
SELECT * FROM posts;

-- name: InsertPost :one
INSERT INTO posts (id, user_id, content)
VALUES (gen_random_uuid(), $1, $2)
RETURNING *;

-- name: FindPostByID :one
SELECT * FROM posts
WHERE id = $1;

-- name: FindPostJoinedByUser :one
SELECT 
  u.id          AS user_id,
  u.name        AS user_name,
  u.email       AS user_email,
  u.created_at  AS user_created_at,
  u.updated_at  AS user_updated_at,
  p.id          AS post_id,
  p.user_id     AS post_user_id,
  p.content     AS post_content,
  p.created_at  AS post_created_at,
  p.updated_at  AS post_updated_at
FROM posts p
JOIN users u ON u.id = p.user_id
WHERE p.id = $1;
