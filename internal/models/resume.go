package models

// ResumeSectionKind identifies which resume bucket an entry belongs to.
type ResumeSectionKind string

const (
	ResumeKindExperience ResumeSectionKind = "experience"
	ResumeKindEducation  ResumeSectionKind = "education"
	ResumeKindActivity   ResumeSectionKind = "activity"
)

// ResumeSection is a labeled group of resume entries.
type ResumeSection struct {
	ID        int64             `json:"id"`
	Kind      ResumeSectionKind `json:"kind"`
	Title     string            `json:"title"`
	SortOrder int               `json:"sort_order"`
	Entries   []ResumeEntry     `json:"entries"`
}

// ResumeEntry is a single role, school, or activity under a section.
type ResumeEntry struct {
	ID        int64  `json:"id"`
	SectionID int64  `json:"section_id"`
	Org       string `json:"org"`
	Role      string `json:"role"`
	Location  string `json:"location"`
	Period    string `json:"period"`
	BodyMD    string `json:"body_md"`
	BodyHTML  string `json:"body_html"`
	Tech      string `json:"tech"`
	SortOrder int    `json:"sort_order"`
}
