/** Full blog post as returned by detail and admin APIs. */
export interface Post {
  id: number;
  slug: string;
  title: string;
  summary: string;
  content_md: string;
  content_html: string;
  published: boolean;
  created_at: string;
  updated_at: string;
  published_at: string | null;
}

/** Public list DTO without full markdown/HTML bodies. */
export interface PostSummary {
  id: number;
  slug: string;
  title: string;
  summary: string;
  created_at: string;
  updated_at: string;
  published_at: string | null;
}
