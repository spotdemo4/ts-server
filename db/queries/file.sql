-- name: GetFile :one
SELECT
    id,
    name,
    data,
    user_id
FROM file
WHERE
    id = @id
    AND
    user_id = @user_id
LIMIT 1;

-- name: InsertFile :one
INSERT INTO file (
    name,
    data,
    user_id
) VALUES (
    @name,
    @data,
    @user_id
)
RETURNING id;

-- name: UpdateFile :exec
UPDATE file
SET
    name = COALESCE(sqlc.narg('name'), name),
    data = COALESCE(sqlc.narg('data'), data)
WHERE
    id = @id
    AND
    user_id = @user_id;

-- name: DeleteFile :exec
DELETE FROM file
WHERE
    id = @id
    AND
    user_id = @user_id;
