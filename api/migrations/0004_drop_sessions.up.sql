-- Drop the cookie-session table. It only served the old Astro SSR frontend;
-- the Vue SPA and native clients authenticate via bearer tokens + the
-- refresh_tokens table (migration 0002). No live code reads it anymore.
DROP TABLE IF EXISTS sessions CASCADE;
