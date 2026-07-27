package models

// NavItem is a primary navigation link from site settings.
type NavItem struct {
	Label string `json:"label"`
	Path  string `json:"path"`
}

// Contact holds public contact links from site settings.
type Contact struct {
	Email    string `json:"email"`
	GitHub   string `json:"github"`
	LinkedIn string `json:"linkedin"`
}

// PublicSettings is the public-safe settings payload.
type PublicSettings struct {
	SiteName        string    `json:"site_name"`
	MetaDescription string    `json:"meta_description"`
	RSSTitle        string    `json:"rss_title"`
	RSSDescription  string    `json:"rss_description"`
	Monogram        string    `json:"monogram"`
	Nav             []NavItem `json:"nav"`
	Contact         Contact   `json:"contact"`
}

// Page is a CMS page document keyed by slug.
type Page struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	MetaDescription string `json:"meta_description"`
	BodyJSON        string `json:"body_json"`
}

// WorkItem is a selected engineering artefact.
type WorkItem struct {
	ID        int64    `json:"id"`
	Name      string   `json:"name"`
	OneLiner  string   `json:"one_liner"`
	Body      string   `json:"body"`
	Stack     []string `json:"stack"`
	Status    string   `json:"status"`
	Href      string   `json:"href"`
	SortOrder int      `json:"sort_order"`
}

// StudioPiece is a catalogue entry for the studio page.
type StudioPiece struct {
	ID           int64  `json:"id"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Year         string `json:"year"`
	Medium       string `json:"medium"`
	Caption      string `json:"caption"`
	ImageMediaID *int64 `json:"image_media_id"`
	ImageURL     string `json:"image_url,omitempty"`
	SortOrder    int    `json:"sort_order"`
	Published    bool   `json:"published"`
}

// MediaAsset is an uploaded file metadata row.
type MediaAsset struct {
	ID           int64  `json:"id"`
	StoredName   string `json:"stored_name"`
	OriginalName string `json:"original_name"`
	Mime         string `json:"mime"`
	ByteSize     int64  `json:"byte_size"`
	CreatedAt    string `json:"created_at"`
	URL          string `json:"url"`
}

// ResumeHeader is the resume page chrome (from pages.slug=resume).
type ResumeHeader struct {
	Eyebrow    string `json:"eyebrow"`
	Headline   string `json:"headline"`
	Blurb      string `json:"blurb"`
	PDFMediaID *int64 `json:"pdf_media_id"`
	PDFURL     string `json:"pdf_url,omitempty"`
}

// Resume is the grouped public resume response including header chrome.
type Resume struct {
	Header   ResumeHeader    `json:"header"`
	Sections []ResumeSection `json:"sections"`
}
