-- migrate:up
CREATE TABLE credential (
    cred_id TEXT PRIMARY KEY NOT NULL,
    cred_public_key BLOB NOT NULL,
    sign_count INTEGER NOT NULL,
    transports TEXT,
    user_verified BOOLEAN,
    backup_eligible BOOLEAN,
    backup_state BOOLEAN,
    attestation_object BLOB,
    attestation_client_data BLOB,
    created_at DATETIME NOT NULL,
    last_used DATETIME NOT NULL,
    user_id INTEGER NOT NULL,

    FOREIGN KEY (user_id) REFERENCES user (id)
);
ALTER TABLE user ADD webauthn_id TEXT NOT NULL;

-- migrate:down
DROP TABLE credential;
ALTER TABLE user DROP COLUMN webauthn_id;
