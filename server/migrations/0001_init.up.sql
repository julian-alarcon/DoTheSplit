CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- users.email_verified_at: null while a registration is unverified; non-null
-- after the user submits the 6-digit code (or auto-set when SMTP was
-- unconfigured at register time so the bootstrap admin isn't stuck).
-- users.notification_prefs: per-event email opt-in flags. JSONB so new keys
-- can be added without further migrations. Absent key means "off".
CREATE TABLE users (
    id                    UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_hash            BYTEA NOT NULL,
    email_encrypted       BYTEA NOT NULL,
    display_name          TEXT  NOT NULL,
    password_hash         TEXT  NOT NULL,
    deleted_at            TIMESTAMPTZ,
    avatar                BYTEA,
    avatar_updated_at     TIMESTAMPTZ,
    week_start            SMALLINT NOT NULL DEFAULT 1,
    role                  TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user','admin')),
    email_verified_at     TIMESTAMPTZ,
    notification_prefs    JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_hash_active_key
    ON users (email_hash)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_users_role_admin_active
    ON users (role)
    WHERE role = 'admin' AND deleted_at IS NULL;

-- Rotating refresh tokens for bearer-token (SPA / Capacitor) auth. Access
-- tokens are stateless JWTs verified by signature; only refresh tokens are
-- persisted so they can be rotated and revoked. We store the SHA-256 hash of
-- the token, never the plaintext.
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

CREATE TABLE groups (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    created_by       UUID NOT NULL REFERENCES users(id),
    default_currency CHAR(3) NOT NULL DEFAULT 'EUR',
    default_split    JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE group_members (
    group_id              UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id               UUID NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    joined_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    -- High-water mark for the unread-activity badge: the newest activity_events
    -- timestamp this member has seen. NULL = never opened the log (all activity
    -- counts as unread). Set to now() when the member opens the activity log.
    last_read_activity_at TIMESTAMPTZ,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_group_members_user_id ON group_members(user_id);

CREATE TABLE categories (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        TEXT NOT NULL UNIQUE,
    label       TEXT NOT NULL,
    sort        INTEGER NOT NULL DEFAULT 0,
    group_label TEXT NOT NULL
);

INSERT INTO categories (slug, label, sort, group_label) VALUES
    -- Entertainment
    ('books',              'Books',              110, 'Entertainment'),
    ('concerts',           'Concerts',           120, 'Entertainment'),
    ('games',              'Games',              130, 'Entertainment'),
    ('hobbies',            'Hobbies',            140, 'Entertainment'),
    ('movies',             'Movies',             150, 'Entertainment'),
    ('music',              'Music',              160, 'Entertainment'),
    ('sports',             'Sports',             170, 'Entertainment'),
    ('theater',            'Theater',            180, 'Entertainment'),
    -- Food & drink
    ('snacks',             'Snacks',             220, 'Food & drink'),
    ('dining_out',         'Dining out',         230, 'Food & drink'),
    ('liquor',             'Liquor',             240, 'Food & drink'),
    -- Home
    ('groceries',          'Groceries',          305, 'Home'),
    ('rent',               'Rent',               310, 'Home'),
    ('mortgage',           'Mortgage',           320, 'Home'),
    ('electronics',        'Electronics',        330, 'Home'),
    ('furniture',          'Furniture',          340, 'Home'),
    ('household_supplies', 'Household supplies', 350, 'Home'),
    ('maintenance',        'Maintenance',        360, 'Home'),
    ('cleaning',           'Cleaning',           370, 'Home'),
    ('pets',               'Pets',               380, 'Home'),
    ('services',           'Services',           390, 'Home'),
    -- Life
    ('childcare',          'Childcare',          410, 'Life'),
    ('clothing',           'Clothing',           420, 'Life'),
    ('gym',                'Gym',                425, 'Life'),
    ('education',          'Education',          430, 'Life'),
    ('gifts',              'Gifts',              440, 'Life'),
    ('insurance',          'Insurance',          450, 'Life'),
    ('medical',            'Medical expenses',   460, 'Life'),
    ('taxes',              'Taxes',              470, 'Life'),
    ('loan',               'Loan',               480, 'Life'),
    ('hotel',              'Hotel',              490, 'Life'),
    ('legal',              'Legal',              495, 'Life'),
    ('real_estate',        'Real estate',        498, 'Life'),
    -- Transport
    ('bicycle',            'Bicycle',            510, 'Transport'),
    ('bus',                'Bus',                520, 'Transport'),
    ('car',                'Car',                530, 'Transport'),
    ('fuel',               'Gas / Fuel',         540, 'Transport'),
    ('parking',            'Parking',            550, 'Transport'),
    ('plane',              'Plane',              560, 'Transport'),
    ('taxi',               'Taxi',               570, 'Transport'),
    ('train',              'Train',              580, 'Transport'),
    ('special',            'Special',            590, 'Transport'),
    -- Utilities
    ('electricity',        'Electricity',        610, 'Utilities'),
    ('heating_gas',        'Heating / Gas',      620, 'Utilities'),
    ('internet',           'Internet',           630, 'Utilities'),
    ('phone',              'Phone',              640, 'Utilities'),
    ('trash',              'Trash',              650, 'Utilities'),
    ('tv',                 'TV',                 660, 'Utilities'),
    ('water',              'Water',              670, 'Utilities'),
    -- No category
    ('other',              'Other',              999, 'No category');

CREATE TABLE expenses (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id     UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    payer_id     UUID NOT NULL REFERENCES users(id),
    created_by   UUID NOT NULL REFERENCES users(id),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    currency     CHAR(3) NOT NULL DEFAULT 'USD',
    description  TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    incurred_at  TIMESTAMPTZ NOT NULL,
    category_id  UUID NOT NULL REFERENCES categories(id),
    -- Links a materialized expense back to the recurring template that spawned
    -- it. Nullable: manual expenses have no template. FK + index are added after
    -- recurring_expenses is created below.
    recurring_expense_id UUID,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ
);
CREATE INDEX idx_expenses_group_incurred
    ON expenses (group_id, incurred_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE splits (
    expense_id  UUID NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    user_id     UUID NOT NULL REFERENCES users(id),
    share_cents BIGINT NOT NULL CHECK (share_cents >= 0),
    PRIMARY KEY (expense_id, user_id)
);

CREATE TABLE settlements (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id     UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    from_user    UUID NOT NULL REFERENCES users(id),
    to_user      UUID NOT NULL REFERENCES users(id),
    amount_cents BIGINT NOT NULL CHECK (amount_cents > 0),
    note         TEXT NOT NULL DEFAULT '',
    settled_at   TIMESTAMPTZ NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at   TIMESTAMPTZ,
    CHECK (from_user <> to_user)
);
CREATE INDEX idx_settlements_group_settled
    ON settlements (group_id, settled_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE recurring_expenses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id        UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    payer_id        UUID NOT NULL REFERENCES users(id),
    amount_cents    BIGINT NOT NULL CHECK (amount_cents > 0),
    currency        CHAR(3) NOT NULL DEFAULT 'USD',
    description     TEXT NOT NULL,
    mode            TEXT NOT NULL CHECK (mode IN ('equal','exact','percent','shares')),
    split_template  JSONB NOT NULL,
    cadence         TEXT NOT NULL CHECK (cadence IN ('daily','weekly','biweekly','monthly','yearly')),
    category_id     UUID NOT NULL REFERENCES categories(id),
    next_run_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE INDEX idx_recurring_next_run
    ON recurring_expenses (next_run_at)
    WHERE deleted_at IS NULL;

-- expenses.recurring_expense_id FK + index, deferred to here because
-- recurring_expenses must exist first.
ALTER TABLE expenses
    ADD CONSTRAINT expenses_recurring_expense_id_fkey
    FOREIGN KEY (recurring_expense_id) REFERENCES recurring_expenses(id);
CREATE INDEX idx_expenses_recurring
    ON expenses (recurring_expense_id)
    WHERE recurring_expense_id IS NOT NULL;

CREATE TABLE expense_revisions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id UUID NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    edited_by  UUID NOT NULL REFERENCES users(id),
    edited_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    field      TEXT NOT NULL CHECK (field IN ('description', 'amount_cents', 'category_id', 'payer_id', 'splits', 'incurred_at', 'notes')),
    old_value  TEXT NOT NULL,
    new_value  TEXT NOT NULL
);
CREATE INDEX idx_expense_revisions_expense_edited
    ON expense_revisions (expense_id, edited_at ASC);

-- SMTP configuration is a single, mutable, instance-wide row. The
-- `id BOOLEAN PRIMARY KEY DEFAULT true CHECK (id)` enforces "exactly one row".
CREATE TABLE smtp_config (
    id                  BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    host                TEXT NOT NULL,
    port                INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    username            TEXT,
    password_encrypted  BYTEA,
    from_address        TEXT NOT NULL,
    tls_mode            TEXT NOT NULL CHECK (tls_mode IN ('none','starttls','tls')),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_by          UUID REFERENCES users(id)
);

-- Append-only log of admin actions. Target FKs are SET NULL on delete so
-- removing a user/group does not block historical rows. Metadata is JSONB;
-- never log plaintext emails or passwords.
CREATE TABLE admin_audit (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id   UUID NOT NULL REFERENCES users(id),
    target_user_id  UUID REFERENCES users(id) ON DELETE SET NULL,
    target_group_id UUID REFERENCES groups(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,
    ip              TEXT,
    user_agent      TEXT,
    success         BOOLEAN NOT NULL,
    metadata        JSONB,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_admin_audit_actor_created  ON admin_audit (actor_user_id, created_at DESC);
CREATE INDEX idx_admin_audit_action_created ON admin_audit (action, created_at DESC);

-- email_verification_tokens: SHA-256 of a 6-digit code. Purpose discriminates
-- register vs change_email vs password_reset (last is for the future). For
-- change_email we cache the prospective new email's hash + ciphertext until
-- confirm so the user keeps logging in with the old address until it's done.
CREATE TABLE email_verification_tokens (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id         UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose         TEXT NOT NULL CHECK (purpose IN ('register','change_email','password_reset')),
    code_hash       BYTEA NOT NULL,
    new_email_hash  BYTEA,
    new_email_enc   BYTEA,
    attempts        SMALLINT NOT NULL DEFAULT 0,
    expires_at      TIMESTAMPTZ NOT NULL,
    consumed_at     TIMESTAMPTZ,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_evt_user_purpose_active
    ON email_verification_tokens (user_id, purpose)
    WHERE consumed_at IS NULL;

-- email_outbox: outbound queue drained by the worker every minute. Stores
-- the recipient email AES-GCM-encrypted (same EmailCipher as users.email)
-- so plaintext addresses never sit at rest in this table.
CREATE TABLE email_outbox (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    to_email_enc    BYTEA NOT NULL,
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL,
    template        TEXT NOT NULL,
    attempts        SMALLINT NOT NULL DEFAULT 0,
    last_error      TEXT,
    sent_at         TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_outbox_pending
    ON email_outbox (next_attempt_at)
    WHERE sent_at IS NULL AND attempts < 5;

-- App-wide install state. Single-row enforcement same as smtp_config. The
-- token_hash is SHA-256 of a freshly-generated random secret; cleartext is
-- never persisted. completed_at is null while first-run setup is pending and
-- becomes non-null when /v1/setup/admin succeeds.
CREATE TABLE app_setup (
    id                  BOOLEAN PRIMARY KEY DEFAULT true CHECK (id),
    token_hash          BYTEA NOT NULL,
    token_generated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at        TIMESTAMPTZ,
    completed_by        UUID REFERENCES users(id) ON DELETE SET NULL
);

-- Append-only feed of group actions. group_id cascades like every other
-- group-scoped table. actor_id is nullable (NULL = worker/system) and is a
-- plain FK because users are soft-deleted, never hard-deleted. expense_id /
-- settlement_id are plain nullable FKs with NO cascade: their targets are
-- soft-deleted (deleted_at), so the row always survives and a `*.deleted`
-- event can still resolve the description it points at.
CREATE TABLE activity_events (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id      UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    actor_id      UUID REFERENCES users(id),
    action        TEXT NOT NULL CHECK (action IN (
                      'expense.created','expense.updated','expense.deleted','expense.restored',
                      'settlement.created','settlement.updated','settlement.deleted','settlement.restored')),
    expense_id    UUID REFERENCES expenses(id),
    settlement_id UUID REFERENCES settlements(id),
    metadata      JSONB NOT NULL DEFAULT '{}'::jsonb,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_activity_events_group_keyset
    ON activity_events (group_id, created_at DESC, id DESC);

-- Real-time fan-out: every insert into the append-only feed (from the API
-- request path, the importers, OR the recurring worker) emits a NOTIFY on the
-- 'activity_events' channel. The API holds one LISTEN connection and pushes a
-- minimal signal (IDs only, well under the 8 KB payload limit) to subscribed
-- SSE clients, which then re-fetch. pg_notify is released only on COMMIT, so
-- subscribers never see a rolled-back row.
CREATE OR REPLACE FUNCTION notify_activity_event() RETURNS trigger AS $$
BEGIN
    PERFORM pg_notify('activity_events', json_build_object(
        'id',            NEW.id,
        'group_id',      NEW.group_id,
        'actor_id',      NEW.actor_id,
        'action',        NEW.action,
        'expense_id',    NEW.expense_id,
        'settlement_id', NEW.settlement_id,
        'created_at',    NEW.created_at
    )::text);
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER activity_event_notify
    AFTER INSERT ON activity_events
    FOR EACH ROW EXECUTE FUNCTION notify_activity_event();
