-- Rotating refresh tokens for bearer-token (SPA / Capacitor) auth. Access
-- tokens are stateless JWTs verified by signature; only refresh tokens are
-- persisted so they can be rotated and revoked. We store the SHA-256 hash of
-- the token, never the plaintext, mirroring the sessions table.
--
-- replaced_by points at the successor minted when this token was rotated.
-- A presented token whose revoked_at is set (or that has a replaced_by) is a
-- reuse attempt: the caller revokes the whole user's chain and rejects.
CREATE TABLE refresh_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  BYTEA NOT NULL UNIQUE,
    issued_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at  TIMESTAMPTZ NOT NULL,
    revoked_at  TIMESTAMPTZ,
    replaced_by UUID REFERENCES refresh_tokens(id) ON DELETE SET NULL
);
CREATE INDEX idx_refresh_tokens_user_id    ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);
