CREATE TABLE categories (
    id     UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    slug   TEXT NOT NULL UNIQUE,
    label  TEXT NOT NULL,
    emoji  TEXT NOT NULL,
    sort   INTEGER NOT NULL DEFAULT 0
);

INSERT INTO categories (slug, label, emoji, sort) VALUES
    ('groceries',     'Groceries',      '🛒', 10),
    ('food_drink',    'Food & Drink',   '🍽️', 20),
    ('transport',     'Transport',      '🚗', 30),
    ('housing',       'Housing',        '🏠', 40),
    ('utilities',     'Utilities',      '💡', 50),
    ('entertainment', 'Entertainment',  '🎬', 60),
    ('travel',        'Travel',         '✈️', 70),
    ('health',        'Health',         '💊', 80),
    ('shopping',      'Shopping',       '🛍️', 90),
    ('other',         'Other',          '📌', 100);

ALTER TABLE expenses
    ADD COLUMN category_id UUID REFERENCES categories(id);

-- Backfill every existing expense with the "other" category, then make it NOT NULL.
UPDATE expenses SET category_id = (SELECT id FROM categories WHERE slug = 'other');
ALTER TABLE expenses ALTER COLUMN category_id SET NOT NULL;

ALTER TABLE recurring_expenses
    ADD COLUMN category_id UUID REFERENCES categories(id);
UPDATE recurring_expenses SET category_id = (SELECT id FROM categories WHERE slug = 'other');
ALTER TABLE recurring_expenses ALTER COLUMN category_id SET NOT NULL;

CREATE TABLE expense_revisions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    expense_id UUID NOT NULL REFERENCES expenses(id) ON DELETE CASCADE,
    edited_by  UUID NOT NULL REFERENCES users(id),
    edited_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    field      TEXT NOT NULL CHECK (field IN ('description', 'amount_cents', 'category_id')),
    old_value  TEXT NOT NULL,
    new_value  TEXT NOT NULL
);
CREATE INDEX idx_expense_revisions_expense_edited
    ON expense_revisions (expense_id, edited_at ASC);
