-- Per-section accordion: experience can collapse while skills stay open.
ALTER TABLE resume_sections ADD COLUMN accordion INTEGER NOT NULL DEFAULT 0;
