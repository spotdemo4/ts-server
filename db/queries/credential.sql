-- name: GetCredential :one
SELECT
    cred_id,
    cred_public_key,
    sign_count,
    transports,
    user_verified,
    backup_eligible,
    backup_state,
    attestation_object,
    attestation_client_data,
    created_at,
    last_used,
    user_id
FROM credential
WHERE
    cred_id = @id
    AND
    user_id = @user_id
LIMIT 1;

-- name: GetCredentials :many
SELECT
    cred_id,
    cred_public_key,
    sign_count,
    transports,
    user_verified,
    backup_eligible,
    backup_state,
    attestation_object,
    attestation_client_data,
    created_at,
    last_used,
    user_id
FROM credential
WHERE user_id = @user_id;

-- name: InsertCredential :exec
INSERT INTO credential (
    cred_id,
    cred_public_key,
    sign_count,
    transports,
    user_verified,
    backup_eligible,
    backup_state,
    attestation_object,
    attestation_client_data,
    created_at,
    last_used,
    user_id
) VALUES (
    @cred_id,
    @cred_public_key,
    @sign_count,
    @transports,
    @user_verified,
    @backup_eligible,
    @backup_state,
    @attestation_object,
    @attestation_client_data,
    @created_at,
    @last_used,
    @user_id
);

-- name: UpdateCredential :exec
UPDATE credential
SET
    last_used = COALESCE(sqlc.narg('last_used'), last_used),
    sign_count = COALESCE(sqlc.narg('sign_count'), sign_count)
WHERE
    cred_id = @id
    AND
    user_id = @user_id;

-- name: DeleteCredential :exec
DELETE FROM credential
WHERE
    cred_id = @id
    AND
    user_id = @user_id;
