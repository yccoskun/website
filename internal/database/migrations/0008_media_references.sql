CREATE TABLE media_references (
  post_id  INTEGER NOT NULL REFERENCES posts(id) ON DELETE CASCADE,
  media_id INTEGER NOT NULL REFERENCES media_assets(id) ON DELETE CASCADE,
  PRIMARY KEY (post_id, media_id)
);
CREATE INDEX idx_media_references_media_id ON media_references(media_id);
