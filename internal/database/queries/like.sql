-- name: FindAllLikes :many
SELECT * FROM likes;

-- name: InsertLike :one
INSERT INTO likes (user_id, post_id)
VALUES ($1, $2)
RETURNING *;

-- name: DeleteLike :one
DELETE FROM likes
WHERE user_id = $1 AND post_id = $2
RETURNING *;    



-- name: FindLikesByPost :many
SELECT 
  l.user_id,
  l.post_id,
  l.created_at,
  u.id         AS user_id,
  u.name       AS user_name,
  u.email      AS user_email,
  u.created_at AS user_created_at,
  u.updated_at AS user_updated_at
FROM likes l
JOIN users u ON u.id = l.user_id
WHERE l.post_id = $1;

-- name: CountLikesByPost :one
SELECT count(*) 
FROM likes
WHERE post_id = $1;
