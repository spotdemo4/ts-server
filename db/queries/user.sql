-- name: GetUser :one
SELECT
    id,
    username,
    password,
    profile_picture_id,
    webauthn_id
FROM user
WHERE
    id = @id
LIMIT 1;

-- name: GetUserbyUsername :one
SELECT
    id,
    username,
    password,
    profile_picture_id,
    webauthn_id
FROM user
WHERE
    username = @username
LIMIT 1;

-- name: InsertUser :one
INSERT INTO user (
    username,
    password,
    webauthn_id
) VALUES (
    @username,
    @password,
    @webauthn_id
)
RETURNING id;

-- name: UpdateUser :exec
UPDATE user
SET
    username = COALESCE(sqlc.narg('username'), username),
    password = COALESCE(sqlc.narg('password'), password),
    profile_picture_id = COALESCE(
        sqlc.narg('profile_picture_id'),
        profile_picture_id
    )
WHERE id = @id;

-- name: DeleteUser :exec
DELETE FROM user
WHERE id = @id;
