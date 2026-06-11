-- Revert to the original action set. Any restore events must be removed first
-- or the new CHECK would fail to validate existing rows.
DELETE FROM activity_events WHERE action IN ('expense.restored','settlement.restored');
ALTER TABLE activity_events DROP CONSTRAINT activity_events_action_check;
ALTER TABLE activity_events ADD CONSTRAINT activity_events_action_check
    CHECK (action IN (
        'expense.created','expense.updated','expense.deleted',
        'settlement.created','settlement.updated','settlement.deleted'));
