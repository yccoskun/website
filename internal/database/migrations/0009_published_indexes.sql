CREATE INDEX idx_posts_published_list
  ON posts(published, COALESCE(published_at, created_at), id);

CREATE INDEX idx_studio_pieces_published_list
  ON studio_pieces(published, sort_order, id);

CREATE INDEX idx_studio_pieces_image_media
  ON studio_pieces(image_media_id, published);
