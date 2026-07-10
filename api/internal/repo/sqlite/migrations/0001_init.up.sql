-- SQLite schema, translated from migrations/0001_init.up.sql.
--
-- Cross-engine mapping vs the Postgres original:
--   UUID           -> TEXT   (google/uuid string form; generated in Go)
--   BYTEA          -> BLOB
--   BIGINT/SMALLINT-> INTEGER
--   TIMESTAMPTZ    -> TEXT   (RFC3339Nano UTC; generated in Go)
--   JSONB          -> TEXT   (marshalled in Go; json_extract() for reads)
--   BOOLEAN        -> INTEGER (0/1)
--   CHAR(3)        -> TEXT
-- No gen_random_uuid()/now() defaults: ids and timestamps are supplied by the
-- Go layer so both engines behave identically. No pg_notify trigger: the
-- SQLite store publishes activity events to the in-process hub after commit.
-- Foreign keys are enforced only when 'PRAGMA foreign_keys=ON' is set on the
-- connection (the store DSN sets it); the group-delete cascade depends on it.

CREATE TABLE users (
    id                    TEXT PRIMARY KEY,
    email_hash            BLOB NOT NULL,
    email_encrypted       BLOB NOT NULL,
    display_name          TEXT NOT NULL,
    password_hash         TEXT NOT NULL,
    deleted_at            TEXT,
    avatar                BLOB,
    avatar_updated_at     TEXT,
    week_start            INTEGER NOT NULL DEFAULT 1,
    role                  TEXT NOT NULL DEFAULT 'user' CHECK (role IN ('user','admin')),
    email_verified_at     TEXT,
    notification_prefs    TEXT NOT NULL DEFAULT '{}',
    created_at            TEXT NOT NULL
);
CREATE UNIQUE INDEX users_email_hash_active_key
    ON users (email_hash)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_users_role_admin_active
    ON users (role)
    WHERE role = 'admin' AND deleted_at IS NULL;

CREATE TABLE refresh_tokens (
    id          TEXT PRIMARY KEY,
    user_id     TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash  BLOB NOT NULL UNIQUE,
    issued_at   TEXT NOT NULL,
    expires_at  TEXT NOT NULL,
    revoked_at  TEXT,
    replaced_by TEXT REFERENCES refresh_tokens(id) ON DELETE SET NULL
);
CREATE INDEX idx_refresh_tokens_user_id    ON refresh_tokens(user_id);
CREATE INDEX idx_refresh_tokens_expires_at ON refresh_tokens(expires_at);

CREATE TABLE groups (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL,
    created_by       TEXT NOT NULL REFERENCES users(id),
    default_currency TEXT NOT NULL DEFAULT 'EUR',
    default_split    TEXT,
    created_at       TEXT NOT NULL
);

CREATE TABLE group_members (
    group_id              TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id               TEXT NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    joined_at             TEXT NOT NULL,
    last_read_activity_at TEXT,
    PRIMARY KEY (group_id, user_id)
);
CREATE INDEX idx_group_members_user_id ON group_members(user_id);

CREATE TABLE categories (
    id          TEXT PRIMARY KEY,
    slug        TEXT NOT NULL UNIQUE,
    label       TEXT NOT NULL,
    sort        INTEGER NOT NULL DEFAULT 0,
    group_label TEXT NOT NULL
);

INSERT INTO categories (id, slug, label, sort, group_label) VALUES
    ('11095f9f-e5d7-5c5e-b217-91e301fded2f', 'books', 'Books', 110, 'Entertainment'),
    ('76025b22-9d81-52db-93e6-0b29f1cf9188', 'concerts', 'Concerts', 120, 'Entertainment'),
    ('249ddbfa-99c7-5ad3-988f-4db8ff730b69', 'games', 'Games', 130, 'Entertainment'),
    ('30f071a8-50ae-50e9-bacb-e872548316bf', 'hobbies', 'Hobbies', 140, 'Entertainment'),
    ('13104551-ffde-5a64-8690-23b21ee372b7', 'movies', 'Movies', 150, 'Entertainment'),
    ('9fce4546-db5e-554d-aede-961d0ccf49d5', 'music', 'Music', 160, 'Entertainment'),
    ('925ac623-432b-5bae-989d-bffa2dfa80de', 'sports', 'Sports', 170, 'Entertainment'),
    ('fbb2196e-c58a-55d3-92f2-fc6988d28805', 'theater', 'Theater', 180, 'Entertainment'),
    ('710785dd-5686-5b94-a320-d75b6bf9b59d', 'snacks', 'Snacks', 220, 'Food & drink'),
    ('b26494d9-d70b-59a0-b191-aca019ad4900', 'dining_out', 'Dining out', 230, 'Food & drink'),
    ('2ce15d53-9ebf-5764-a0eb-6ff3e4688a42', 'liquor', 'Liquor', 240, 'Food & drink'),
    ('bfc9019d-bdfe-5697-9516-325341dc978b', 'groceries', 'Groceries', 305, 'Home'),
    ('6dcb5401-f283-5cb3-b4f1-8b6b4d4bfa63', 'rent', 'Rent', 310, 'Home'),
    ('a8945543-e181-508e-8b1a-6cff74667df5', 'mortgage', 'Mortgage', 320, 'Home'),
    ('05fec87b-0926-5ab4-8afa-f2cd47d6153e', 'electronics', 'Electronics', 330, 'Home'),
    ('785c8f96-47c5-5671-a91e-b2956b968d32', 'furniture', 'Furniture', 340, 'Home'),
    ('62f65c07-b4ef-55ea-8333-bf652152344b', 'household_supplies', 'Household supplies', 350, 'Home'),
    ('6b66525d-d0a4-51f6-bf59-2d6eb0313e5b', 'maintenance', 'Maintenance', 360, 'Home'),
    ('d3e197ac-c288-5ce3-9cfc-cfde01967eb1', 'cleaning', 'Cleaning', 370, 'Home'),
    ('3a2dbb6d-96ee-5624-bfea-da536f2a4682', 'pets', 'Pets', 380, 'Home'),
    ('a895b953-b448-5dca-bc29-49aa410787ab', 'services', 'Services', 390, 'Home'),
    ('235c7cf9-3c66-57ea-974c-381d245adf37', 'childcare', 'Childcare', 410, 'Life'),
    ('dfc37053-8c0a-5d5e-875e-6d38c265f0c0', 'clothing', 'Clothing', 420, 'Life'),
    ('4ee526d4-3ad9-5517-8985-0650372772cc', 'gym', 'Gym', 425, 'Life'),
    ('249c17ed-01d8-5a0c-9a57-ff08e3969ab4', 'education', 'Education', 430, 'Life'),
    ('ce4ebb80-da06-5c66-bad7-951f8248fd05', 'gifts', 'Gifts', 440, 'Life'),
    ('f512a8d8-e16c-5c6c-a986-bbb4d5dd1273', 'insurance', 'Insurance', 450, 'Life'),
    ('d19375ac-a245-5d15-95f6-cb06e13cd7b1', 'medical', 'Medical expenses', 460, 'Life'),
    ('c2d6292c-cba2-54f1-965b-e41d234c301b', 'taxes', 'Taxes', 470, 'Life'),
    ('bd6d0710-9605-5e6c-9891-c37871883d31', 'loan', 'Loan', 480, 'Life'),
    ('d5e7a386-743e-54e9-abf1-cd081715628a', 'hotel', 'Hotel', 490, 'Life'),
    ('63ac590e-725b-58f4-94bd-7cbeea1e9b1a', 'legal', 'Legal', 495, 'Life'),
    ('0537806a-7d30-56bc-bbf4-bcb5aaab40dd', 'real_estate', 'Real estate', 498, 'Life'),
    ('bfbb80e0-fd56-59a2-ae83-d4a4725267b6', 'bicycle', 'Bicycle', 510, 'Transport'),
    ('89851099-5d0d-5111-9bc5-67d262d23a6b', 'bus', 'Bus', 520, 'Transport'),
    ('5ea4a070-d748-584f-a45f-1549a2e3ab30', 'car', 'Car', 530, 'Transport'),
    ('fa5e1ec7-83ae-50b7-b762-909588bfaba1', 'fuel', 'Gas / Fuel', 540, 'Transport'),
    ('141348f3-549e-5383-b59e-061ad990140e', 'parking', 'Parking', 550, 'Transport'),
    ('3a197729-83a4-5c38-ace1-29474bf4e71f', 'plane', 'Plane', 560, 'Transport'),
    ('0e90b6f1-73fc-5416-8731-5d3c58a6b054', 'taxi', 'Taxi', 570, 'Transport'),
    ('65fcf442-8608-5b40-89eb-6299e94469d4', 'train', 'Train', 580, 'Transport'),
    ('458f8683-a42a-5f48-b107-ca3a90e1d151', 'special', 'Special', 590, 'Transport'),
    ('284b7870-c7d2-54da-93d3-3799aa4a42ee', 'electricity', 'Electricity', 610, 'Utilities'),
    ('ada31834-9f04-5af6-83b7-89b97c5dbfe4', 'heating_gas', 'Heating / Gas', 620, 'Utilities'),
    ('39a7e698-354c-5161-b4a2-b47fed3c5c02', 'internet', 'Internet', 630, 'Utilities'),
    ('f6941a0d-8898-5ae0-811b-939e44f05306', 'phone', 'Phone', 640, 'Utilities'),
    ('6008e4be-d138-5547-957e-cefab550e944', 'trash', 'Trash', 650, 'Utilities'),
    ('2db23a6e-6f94-5110-9bac-6b76b771898a', 'tv', 'TV', 660, 'Utilities'),
    ('33b469c1-fc34-51f8-8f2b-9aa843d9be51', 'water', 'Water', 670, 'Utilities'),
    ('d36d5f1a-e840-5248-993c-0d8b79cc3dd5', 'other', 'Other', 999, 'No category');

CREATE TABLE recurring_expenses (
    id              TEXT PRIMARY KEY,
    group_id        TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    payer_id        TEXT NOT NULL REFERENCES users(id),
    amount_cents    INTEGER NOT NULL CHECK (amount_cents > 0),
    currency        TEXT NOT NULL DEFAULT 'USD',
    description     TEXT NOT NULL,
    mode            TEXT NOT NULL CHECK (mode IN ('equal','exact','percent','shares')),
    split_template  TEXT NOT NULL,
    cadence         TEXT NOT NULL CHECK (cadence IN ('daily','weekly','biweekly','monthly','yearly')),
    category_id     TEXT NOT NULL REFERENCES categories(id),
    next_run_at     TEXT NOT NULL,
    created_at      TEXT NOT NULL,
    deleted_at      TEXT
);
CREATE INDEX idx_recurring_next_run
    ON recurring_expenses (next_run_at)
    WHERE deleted_at IS NULL;

CREATE TABLE expenses (
    id           TEXT PRIMARY KEY,
    group_id     TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    payer_id     TEXT NOT NULL REFERENCES users(id),
    created_by   TEXT NOT NULL REFERENCES users(id),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    currency     TEXT NOT NULL DEFAULT 'USD',
    description  TEXT NOT NULL,
    notes        TEXT NOT NULL DEFAULT '',
    incurred_at  TEXT NOT NULL,
    category_id  TEXT NOT NULL REFERENCES categories(id),
    recurring_expense_id TEXT REFERENCES recurring_expenses(id),
    created_at   TEXT NOT NULL,
    deleted_at   TEXT
);
CREATE INDEX idx_expenses_group_incurred
    ON expenses (group_id, incurred_at DESC)
    WHERE deleted_at IS NULL;
CREATE INDEX idx_expenses_recurring
    ON expenses (recurring_expense_id)
    WHERE recurring_expense_id IS NOT NULL;

CREATE TABLE splits (
    expense_id  TEXT NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    user_id     TEXT NOT NULL REFERENCES users(id),
    share_cents INTEGER NOT NULL CHECK (share_cents >= 0),
    PRIMARY KEY (expense_id, user_id)
);

CREATE TABLE settlements (
    id           TEXT PRIMARY KEY,
    group_id     TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    from_user    TEXT NOT NULL REFERENCES users(id),
    to_user      TEXT NOT NULL REFERENCES users(id),
    amount_cents INTEGER NOT NULL CHECK (amount_cents > 0),
    note         TEXT NOT NULL DEFAULT '',
    settled_at   TEXT NOT NULL,
    created_at   TEXT NOT NULL,
    deleted_at   TEXT,
    CHECK (from_user <> to_user)
);
CREATE INDEX idx_settlements_group_settled
    ON settlements (group_id, settled_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE expense_revisions (
    id         TEXT PRIMARY KEY,
    expense_id TEXT NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    edited_by  TEXT NOT NULL REFERENCES users(id),
    edited_at  TEXT NOT NULL,
    field      TEXT NOT NULL CHECK (field IN ('description', 'amount_cents', 'category_id', 'payer_id', 'splits', 'incurred_at', 'notes')),
    old_value  TEXT NOT NULL,
    new_value  TEXT NOT NULL
);
CREATE INDEX idx_expense_revisions_expense_edited
    ON expense_revisions (expense_id, edited_at ASC);

CREATE TABLE smtp_config (
    id                  INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    host                TEXT NOT NULL,
    port                INTEGER NOT NULL CHECK (port BETWEEN 1 AND 65535),
    username            TEXT,
    password_encrypted  BLOB,
    from_address        TEXT NOT NULL,
    tls_mode            TEXT NOT NULL CHECK (tls_mode IN ('none','starttls','tls')),
    updated_at          TEXT NOT NULL,
    updated_by          TEXT REFERENCES users(id)
);

CREATE TABLE admin_audit (
    id              TEXT PRIMARY KEY,
    actor_user_id   TEXT NOT NULL REFERENCES users(id),
    target_user_id  TEXT REFERENCES users(id) ON DELETE SET NULL,
    target_group_id TEXT REFERENCES groups(id) ON DELETE SET NULL,
    action          TEXT NOT NULL,
    ip              TEXT,
    user_agent      TEXT,
    success         INTEGER NOT NULL,
    metadata        TEXT,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_admin_audit_actor_created  ON admin_audit (actor_user_id, created_at DESC);
CREATE INDEX idx_admin_audit_action_created ON admin_audit (action, created_at DESC);

CREATE TABLE email_verification_tokens (
    id              TEXT PRIMARY KEY,
    user_id         TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    purpose         TEXT NOT NULL CHECK (purpose IN ('register','change_email','password_reset')),
    code_hash       BLOB NOT NULL,
    new_email_hash  BLOB,
    new_email_enc   BLOB,
    attempts        INTEGER NOT NULL DEFAULT 0,
    expires_at      TEXT NOT NULL,
    consumed_at     TEXT,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_evt_user_purpose_active
    ON email_verification_tokens (user_id, purpose)
    WHERE consumed_at IS NULL;

CREATE TABLE email_outbox (
    id              TEXT PRIMARY KEY,
    to_email_enc    BLOB NOT NULL,
    subject         TEXT NOT NULL,
    body            TEXT NOT NULL,
    template        TEXT NOT NULL,
    attempts        INTEGER NOT NULL DEFAULT 0,
    last_error      TEXT,
    sent_at         TEXT,
    next_attempt_at TEXT NOT NULL,
    created_at      TEXT NOT NULL
);
CREATE INDEX idx_outbox_pending
    ON email_outbox (next_attempt_at)
    WHERE sent_at IS NULL AND attempts < 5;

CREATE TABLE app_setup (
    id                  INTEGER PRIMARY KEY DEFAULT 1 CHECK (id = 1),
    token_hash          BLOB NOT NULL,
    token_generated_at  TEXT NOT NULL,
    completed_at        TEXT,
    completed_by        TEXT REFERENCES users(id) ON DELETE SET NULL
);

CREATE TABLE activity_events (
    id            TEXT PRIMARY KEY,
    group_id      TEXT NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    actor_id      TEXT REFERENCES users(id),
    action        TEXT NOT NULL CHECK (action IN (
                      'expense.created','expense.updated','expense.deleted','expense.restored',
                      'settlement.created','settlement.updated','settlement.deleted','settlement.restored')),
    expense_id    TEXT REFERENCES expenses(id),
    settlement_id TEXT REFERENCES settlements(id),
    metadata      TEXT NOT NULL DEFAULT '{}',
    created_at    TEXT NOT NULL
);
CREATE INDEX idx_activity_events_group_keyset
    ON activity_events (group_id, created_at DESC, id DESC);
