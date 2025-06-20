CREATE TABLE IF NOT EXISTS "schema_migrations" (version varchar(128) primary key);
CREATE TABLE user (
    id INTEGER PRIMARY KEY NOT NULL,
    username TEXT NOT NULL,
    password TEXT NOT NULL,
    profile_picture_id INTEGER, webauthn_id TEXT NOT NULL,

    FOREIGN KEY (profile_picture_id) REFERENCES file (id)
);
CREATE TABLE file (
    id INTEGER PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    data BLOB NOT NULL,
    user_id INTEGER NOT NULL,

    FOREIGN KEY (user_id) REFERENCES user (id)
);
CREATE TABLE item (
    id INTEGER PRIMARY KEY NOT NULL,
    name TEXT NOT NULL,
    added DATETIME NOT NULL,
    description TEXT NOT NULL,
    price REAL NOT NULL,
    quantity INTEGER NOT NULL,
    user_id INTEGER NOT NULL,

    FOREIGN KEY (user_id) REFERENCES user (id)
);
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
-- Dbmate schema migrations
INSERT INTO "schema_migrations" (version) VALUES
  ('20250410195416'),
  ('20250418055807');
