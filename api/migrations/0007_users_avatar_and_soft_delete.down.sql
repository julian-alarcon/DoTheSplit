DROP INDEX IF EXISTS users_email_hash_active_key;
ALTER TABLE users ADD CONSTRAINT users_email_hash_key UNIQUE (email_hash);
ALTER TABLE users
    DROP COLUMN IF EXISTS avatar_updated_at,
    DROP COLUMN IF EXISTS avatar,
    DROP COLUMN IF EXISTS deleted_at;
