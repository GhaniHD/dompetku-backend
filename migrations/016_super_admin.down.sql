DROP TABLE IF EXISTS announcements;
DROP TABLE IF EXISTS presence_hours;
DROP TABLE IF EXISTS auth_events;
ALTER TABLE users DROP COLUMN IF EXISTS last_seen_at;
