-- CMS schema only — no editorial content seeds.
-- Content is managed via admin API / import after migrate.

CREATE TABLE site_settings (
  key TEXT PRIMARY KEY,
  value TEXT NOT NULL
);

CREATE TABLE pages (
  slug TEXT PRIMARY KEY,
  title TEXT NOT NULL DEFAULT '',
  meta_description TEXT NOT NULL DEFAULT '',
  body_json TEXT NOT NULL DEFAULT '{}'
);

CREATE TABLE work_items (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  name TEXT NOT NULL,
  one_liner TEXT NOT NULL DEFAULT '',
  body TEXT NOT NULL DEFAULT '',
  stack_json TEXT NOT NULL DEFAULT '[]',
  status TEXT NOT NULL DEFAULT '',
  href TEXT NOT NULL DEFAULT '',
  sort_order INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE media_assets (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  stored_name TEXT NOT NULL,
  original_name TEXT NOT NULL DEFAULT '',
  mime TEXT NOT NULL DEFAULT '',
  byte_size INTEGER NOT NULL DEFAULT 0,
  created_at TEXT NOT NULL DEFAULT (strftime('%Y-%m-%dT%H:%M:%fZ','now'))
);

CREATE TABLE studio_pieces (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  slug TEXT NOT NULL UNIQUE,
  title TEXT NOT NULL,
  year TEXT NOT NULL DEFAULT '',
  medium TEXT NOT NULL DEFAULT '',
  caption TEXT NOT NULL DEFAULT '',
  image_media_id INTEGER REFERENCES media_assets(id) ON DELETE SET NULL,
  sort_order INTEGER NOT NULL DEFAULT 0,
  published INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX idx_work_items_sort ON work_items(sort_order, id);
CREATE INDEX idx_studio_pieces_sort ON studio_pieces(sort_order, id);
