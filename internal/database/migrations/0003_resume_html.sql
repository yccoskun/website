-- Add sanitized HTML body for resume entries (rendered at write time).
ALTER TABLE resume_entries ADD COLUMN body_html TEXT NOT NULL DEFAULT '';
