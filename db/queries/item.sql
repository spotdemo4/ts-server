-- name: GetItem :one
SELECT
    id,
    name,
    added,
    description,
    price,
    quantity,
    user_id
FROM item
WHERE
    id = @id
    AND
    user_id = @user_id
LIMIT 1;

-- name: GetItems :many
SELECT
    id,
    name,
    added,
    description,
    price,
    quantity,
    user_id
FROM item
WHERE
    user_id = @user_id
    AND
    (
        (name LIKE sqlc.narg('name') OR sqlc.narg('name') IS NULL)
        AND
        (added >= sqlc.narg('start') OR sqlc.narg('start') IS NULL)
        AND
        (added <= sqlc.narg('end') OR sqlc.narg('end') IS NULL)
    )
ORDER BY added DESC
LIMIT
    @limit
    OFFSET
    @offset;

-- name: GetItemsCount :one
SELECT COUNT(id)
FROM item
WHERE
    user_id = @user_id
    AND
    (
        (name LIKE sqlc.narg('name') OR sqlc.narg('name') IS NULL)
        AND
        (added >= sqlc.narg('start') OR sqlc.narg('start') IS NULL)
        AND
        (added <= sqlc.narg('end') OR sqlc.narg('end') IS NULL)
    )
LIMIT 1;

-- name: InsertItem :one
INSERT INTO item (
    name,
    added,
    description,
    price,
    quantity,
    user_id
) VALUES (
    @name,
    @added,
    @description,
    @price,
    @quantity,
    @user_id
)
RETURNING id;

-- name: UpdateItem :exec
UPDATE item
SET
    name = COALESCE(sqlc.narg('name'), name),
    description = COALESCE(sqlc.narg('description'), description),
    price = COALESCE(sqlc.narg('price'), price),
    quantity = COALESCE(sqlc.narg('quantity'), quantity)
WHERE
    id = @id
    AND
    user_id = @user_id;

-- name: DeleteItem :exec
DELETE FROM item
WHERE
    id = @id
    AND
    user_id = @user_id;
