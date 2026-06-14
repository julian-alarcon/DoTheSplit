-- Drop the per-user IANA timezone override. Date display now follows the
-- viewer's device timezone unconditionally (Intl.DateTimeFormat in the SPA);
-- the stored override was a no-op after the Astro -> Vue port, so the setting
-- and its column are removed. Timestamps remain TIMESTAMPTZ (UTC instants).
ALTER TABLE users DROP COLUMN timezone;
