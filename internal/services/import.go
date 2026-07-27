package services

import (
	"fmt"
)

// ContentImport is a JSON dump for bootstrap / environment transfer via admin import.
type ContentImport struct {
	Settings map[string]string `json:"settings"`
	Pages    []PageImport      `json:"pages"`
	Work     []WorkInput       `json:"work"`
	Studio   []StudioInput     `json:"studio"`
	Sections []ResumeSectionInput `json:"resume_sections"`
	Entries  []ResumeEntryInput   `json:"resume_entries"`
	// ReplaceResume when true deletes all existing resume sections (cascade entries)
	// before inserting imported sections/entries.
	ReplaceResume bool `json:"replace_resume"`
	// ReplaceWork when true deletes all work items before import.
	ReplaceWork bool `json:"replace_work"`
	// ReplaceStudio when true deletes all studio pieces before import.
	ReplaceStudio bool `json:"replace_studio"`
}

// PageImport is a page row in an import dump.
type PageImport struct {
	Slug            string `json:"slug"`
	Title           string `json:"title"`
	MetaDescription string `json:"meta_description"`
	BodyJSON        string `json:"body_json"`
}

// ImportResult summarizes what was written.
type ImportResult struct {
	SettingsUpserted int `json:"settings_upserted"`
	PagesUpserted    int `json:"pages_upserted"`
	WorkCreated      int `json:"work_created"`
	StudioCreated    int `json:"studio_created"`
	SectionsCreated  int `json:"sections_created"`
	EntriesCreated   int `json:"entries_created"`
}

// ImportService applies or exports a content dump. Media files are not included.
type ImportService struct {
	Settings *SettingsService
	Pages    *PageService
	Work     *WorkService
	Studio   *StudioService
	Resume   *ResumeService
}

// Export builds a ContentImport dump of the current database state.
// Replace flags are set true so re-importing on another environment replaces lists.
// Resume entry section_id values are 1-based indexes into resume_sections (import remap).
// Media binaries are not included; pdf_media_id / image_media_id may need remapping after upload.
func (s *ImportService) Export() (ContentImport, error) {
	out := ContentImport{
		ReplaceResume: true,
		ReplaceWork:   true,
		ReplaceStudio: true,
		Settings:      map[string]string{},
		Pages:         []PageImport{},
		Work:          []WorkInput{},
		Studio:        []StudioInput{},
		Sections:      []ResumeSectionInput{},
		Entries:       []ResumeEntryInput{},
	}

	settings, err := s.Settings.GetAll()
	if err != nil {
		return out, err
	}
	out.Settings = settings

	pages, err := s.Pages.List()
	if err != nil {
		return out, err
	}
	for _, p := range pages {
		out.Pages = append(out.Pages, PageImport{
			Slug:            p.Slug,
			Title:           p.Title,
			MetaDescription: p.MetaDescription,
			BodyJSON:        p.BodyJSON,
		})
	}

	work, err := s.Work.List()
	if err != nil {
		return out, err
	}
	for _, w := range work {
		out.Work = append(out.Work, WorkInput{
			Name: w.Name, OneLiner: w.OneLiner, Body: w.Body,
			Stack: w.Stack, Status: w.Status, Href: w.Href, SortOrder: w.SortOrder,
		})
	}

	studio, err := s.Studio.AdminList()
	if err != nil {
		return out, err
	}
	for _, p := range studio {
		out.Studio = append(out.Studio, StudioInput{
			Slug: p.Slug, Title: p.Title, Year: p.Year, Medium: p.Medium,
			Caption: p.Caption, ImageMediaID: p.ImageMediaID,
			SortOrder: p.SortOrder, Published: p.Published,
		})
	}

	sections, err := s.Resume.ListSections()
	if err != nil {
		return out, err
	}
	sectionIndex := make(map[int64]int64, len(sections))
	for i, sec := range sections {
		out.Sections = append(out.Sections, ResumeSectionInput{
			Kind: sec.Kind, Title: sec.Title, SortOrder: sec.SortOrder, Accordion: sec.Accordion,
		})
		sectionIndex[sec.ID] = int64(i + 1)
	}

	entries, err := s.Resume.AdminListEntries()
	if err != nil {
		return out, err
	}
	for _, e := range entries {
		idx, ok := sectionIndex[e.SectionID]
		if !ok {
			return out, fmt.Errorf("%w: resume entry %d references missing section %d", ErrValidation, e.ID, e.SectionID)
		}
		out.Entries = append(out.Entries, ResumeEntryInput{
			SectionID: idx,
			Org:       e.Org,
			Role:      e.Role,
			Location:  e.Location,
			Period:    e.Period,
			BodyMD:    e.BodyMD,
			Tech:      e.Tech,
			SortOrder: e.SortOrder,
		})
	}

	return out, nil
}

// Apply imports content. Resume entries reference section sort order via SectionID
// as the 1-based index among newly created sections when ReplaceResume is true;
// otherwise SectionID is the real DB id.
func (s *ImportService) Apply(in ContentImport) (ImportResult, error) {
	var out ImportResult

	if len(in.Settings) > 0 {
		if _, err := s.Settings.Upsert(in.Settings); err != nil {
			return out, err
		}
		out.SettingsUpserted = len(in.Settings)
	}

	for _, p := range in.Pages {
		if _, err := s.Pages.Upsert(p.Slug, PageInput{
			Title:           p.Title,
			MetaDescription: p.MetaDescription,
			BodyJSON:        p.BodyJSON,
		}); err != nil {
			return out, fmt.Errorf("page %q: %w", p.Slug, err)
		}
		out.PagesUpserted++
	}

	if in.ReplaceWork {
		items, err := s.Work.List()
		if err != nil {
			return out, err
		}
		for _, w := range items {
			if err := s.Work.Delete(w.ID); err != nil {
				return out, err
			}
		}
	}
	for _, w := range in.Work {
		if _, err := s.Work.Create(w); err != nil {
			return out, fmt.Errorf("work %q: %w", w.Name, err)
		}
		out.WorkCreated++
	}

	if in.ReplaceStudio {
		items, err := s.Studio.AdminList()
		if err != nil {
			return out, err
		}
		for _, p := range items {
			if err := s.Studio.Delete(p.ID); err != nil {
				return out, err
			}
		}
	}
	for _, p := range in.Studio {
		if _, err := s.Studio.Create(p); err != nil {
			return out, fmt.Errorf("studio %q: %w", p.Slug, err)
		}
		out.StudioCreated++
	}

	sectionIDs := make([]int64, 0, len(in.Sections))
	if in.ReplaceResume {
		secs, err := s.Resume.ListSections()
		if err != nil {
			return out, err
		}
		for _, sec := range secs {
			if err := s.Resume.DeleteSection(sec.ID); err != nil {
				return out, err
			}
		}
	}
	for _, sec := range in.Sections {
		created, err := s.Resume.CreateSection(sec)
		if err != nil {
			return out, fmt.Errorf("resume section %q: %w", sec.Title, err)
		}
		sectionIDs = append(sectionIDs, created.ID)
		out.SectionsCreated++
	}

	for _, e := range in.Entries {
		entry := e
		if in.ReplaceResume && len(sectionIDs) > 0 {
			// SectionID in dump is 1-based index into imported sections.
			idx := int(e.SectionID) - 1
			if idx < 0 || idx >= len(sectionIDs) {
				return out, fmt.Errorf("%w: resume entry section_id %d out of range", ErrValidation, e.SectionID)
			}
			entry.SectionID = sectionIDs[idx]
		}
		if _, err := s.Resume.CreateEntry(entry); err != nil {
			return out, fmt.Errorf("resume entry: %w", err)
		}
		out.EntriesCreated++
	}

	return out, nil
}
