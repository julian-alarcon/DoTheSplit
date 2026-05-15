CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE users (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    email_hash        BYTEA NOT NULL,
    email_encrypted   BYTEA NOT NULL,
    display_name      TEXT  NOT NULL,
    password_hash     TEXT  NOT NULL,
    deleted_at        TIMESTAMPTZ,
    avatar            BYTEA,
    avatar_updated_at TIMESTAMPTZ,
    week_start        SMALLINT NOT NULL DEFAULT 1,
    timezone          TEXT,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX users_email_hash_active_key
    ON users (email_hash)
    WHERE deleted_at IS NULL;

CREATE TABLE sessions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash BYTEA NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user_id    ON sessions(user_id);
CREATE INDEX idx_sessions_expires_at ON sessions(expires_at);

CREATE TABLE groups (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT NOT NULL,
    created_by       UUID NOT NULL REFERENCES users(id),
    default_currency CHAR(3) NOT NULL DEFAULT 'EUR',
    default_split    JSONB,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE group_members (
    group_id  UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    user_id   UUID NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    joined_at TIMESTAMPTZ NOT NULL DEFAULT now(),
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
    incurred_at  TIMESTAMPTZ NOT NULL,
    category_id  UUID NOT NULL REFERENCES categories(id),
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

CREATE TABLE expense_revisions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id UUID NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    edited_by  UUID NOT NULL REFERENCES users(id),
    edited_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    field      TEXT NOT NULL CHECK (field IN ('description', 'amount_cents', 'category_id', 'payer_id', 'splits', 'incurred_at')),
    old_value  TEXT NOT NULL,
    new_value  TEXT NOT NULL
);
CREATE INDEX idx_expense_revisions_expense_edited
    ON expense_revisions (expense_id, edited_at ASC);
