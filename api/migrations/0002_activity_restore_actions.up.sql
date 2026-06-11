-- Allow the restore events emitted when a soft-deleted expense or settlement is
-- brought back. The original CHECK only covered created/updated/deleted.
ALTER TABLE activity_events DROP CONSTRAINT activity_events_action_check;
ALTER TABLE activity_events ADD CONSTRAINT activity_events_action_check
    CHECK (action IN (
        'expense.created','expense.updated','expense.deleted','expense.restored',
        'settlement.created','settlement.updated','settlement.deleted','settlement.restored'));
