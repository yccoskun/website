-- posts
CREATE TABLE posts (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  summary TEXT NOT NULL DEFAULT '',
  content_md TEXT NOT NULL DEFAULT '',
  content_html TEXT NOT NULL DEFAULT '',
  published INTEGER NOT NULL DEFAULT 0, -- 0/1
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  updated_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  published_at TEXT -- nullable
);

-- resume_sections
CREATE TABLE resume_sections (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  kind TEXT NOT NULL CHECK (kind IN ('experience','education','activity')),
  title TEXT NOT NULL,
  sort_order INTEGER NOT NULL DEFAULT 0
);

-- resume_entries
CREATE TABLE resume_entries (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  section_id INTEGER NOT NULL REFERENCES resume_sections(id) ON DELETE CASCADE,
  org TEXT NOT NULL DEFAULT '',
  role TEXT NOT NULL DEFAULT '',
  location TEXT NOT NULL DEFAULT '',
  period TEXT NOT NULL DEFAULT '',
  body_md TEXT NOT NULL DEFAULT '',
  tech TEXT NOT NULL DEFAULT '', -- comma-separated for now
  sort_order INTEGER NOT NULL DEFAULT 0
);

-- sessions
CREATE TABLE sessions (
  token_hash TEXT PRIMARY KEY, -- SHA-256 hex of the cookie token
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now')),
  expires_at TEXT NOT NULL
);
CREATE INDEX idx_sessions_expires ON sessions(expires_at);

-- Seed resume section scaffolding so the public resume API has structure
-- before any entries exist.
INSERT INTO resume_sections (kind, title, sort_order) VALUES
  ('experience', 'Experience', 1),
  ('education', 'Education', 2),
  ('activity', 'Activities', 3);
