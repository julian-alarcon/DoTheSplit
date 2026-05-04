CREATE TABLE recurring_expenses (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    group_id        UUID NOT NULL REFERENCES groups(id) ON DELETE CASCADE,
    payer_id        UUID NOT NULL REFERENCES users(id),
    amount_cents    BIGINT NOT NULL CHECK (amount_cents > 0),
    currency        CHAR(3) NOT NULL DEFAULT 'USD',
    description     TEXT NOT NULL,
    mode            TEXT NOT NULL CHECK (mode IN ('equal','exact','percent','shares')),
    split_template  JSONB NOT NULL,
    cadence         TEXT NOT NULL CHECK (cadence IN ('daily','weekly','monthly')),
    next_run_at     TIMESTAMPTZ NOT NULL,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);
CREATE INDEX idx_recurring_next_run
    ON recurring_expenses (next_run_at)
    WHERE deleted_at IS NULL;
