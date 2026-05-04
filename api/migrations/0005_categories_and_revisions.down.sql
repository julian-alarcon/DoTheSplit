DROP TABLE IF EXISTS expense_revisions;
ALTER TABLE recurring_expenses DROP COLUMN IF EXISTS category_id;
ALTER TABLE expenses           DROP COLUMN IF EXISTS category_id;
DROP TABLE IF EXISTS categories;
