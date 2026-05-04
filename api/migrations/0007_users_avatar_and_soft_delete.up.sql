ALTER TABLE users
    ADD COLUMN deleted_at        TIMESTAMPTZ,
    ADD COLUMN avatar            BYTEA,
    ADD COLUMN avatar_updated_at TIMESTAMPTZ;

-- Replace the blanket UNIQUE(email_hash) constraint with a partial unique index
-- so a new user can register with the same email after the original has been
-- soft-deleted (their email_hash is scrambled by the delete, but this also
-- future-proofs accidental collisions).
ALTER TABLE users DROP CONSTRAINT users_email_hash_key;
CREATE UNIQUE INDEX users_email_hash_active_key
    ON users (email_hash)
    WHERE deleted_at IS NULL;
