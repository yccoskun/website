-- Soft session binding hints (SHA-256 hex). NULL/empty = unbound / legacy.
ALTER TABLE sessions ADD COLUMN ua_hash TEXT NULL;
ALTER TABLE sessions ADD COLUMN ip_prefix_hash TEXT NULL;
