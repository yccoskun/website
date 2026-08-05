package models

// Post is a blog post stored in SQLite.
type Post struct {
	ID          int64   `json:"id"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Summary     string  `json:"summary"`
	ContentMD   string  `json:"content_md"`
	ContentHTML string  `json:"content_html"`
	Published   bool    `json:"published"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	PublishedAt *string `json:"published_at"`
}

// PostSummary is the public list DTO without full markdown/HTML bodies.
type PostSummary struct {
	ID          int64   `json:"id"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Summary     string  `json:"summary"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	PublishedAt *string `json:"published_at"`
}

// AdminPostSummary is the admin list DTO: metadata + published, no bodies.
type AdminPostSummary struct {
	ID          int64   `json:"id"`
	Slug        string  `json:"slug"`
	Title       string  `json:"title"`
	Summary     string  `json:"summary"`
	Published   bool    `json:"published"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
	PublishedAt *string `json:"published_at"`
}
