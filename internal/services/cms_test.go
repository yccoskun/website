package services_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/yccoskun/website/internal/database"
	"github.com/yccoskun/website/internal/models"
	"github.com/yccoskun/website/internal/services"
)

func openCMSDB(t *testing.T) *sql.DB {
	t.Helper()
	path := filepath.Join(t.TempDir(), "cms.db")
	db, err := database.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	if err := database.Migrate(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func TestCMSSchemaTablesExist(t *testing.T) {
	db := openCMSDB(t)
	tables := []string{"site_settings", "pages", "work_items", "studio_pieces", "media_assets"}
	for _, name := range tables {
		var n int
		err := db.QueryRow(
			`SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name=?`, name,
		).Scan(&n)
		if err != nil || n != 1 {
			t.Fatalf("table %s missing: err=%v n=%d", name, err, n)
		}
	}
}

func TestSettingsUpsertAndPublic(t *testing.T) {
	db := openCMSDB(t)
	s := services.NewSettingsService(db)
	_, err := s.Upsert(map[string]string{
		"site_name": "Test Site",
		"nav":       `[{"label":"Home","path":"/"}]`,
		"contact":   `{"email":"a@b.c","github":"","linkedin":""}`,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	pub, err := s.Public()
	if err != nil {
		t.Fatalf("public: %v", err)
	}
	if pub.SiteName != "Test Site" {
		t.Fatalf("site_name = %q", pub.SiteName)
	}
	if len(pub.Nav) != 1 || pub.Nav[0].Path != "/" {
		t.Fatalf("nav = %+v", pub.Nav)
	}
	if pub.Contact.Email != "a@b.c" {
		t.Fatalf("contact = %+v", pub.Contact)
	}
}

func TestPageUpsertValidatesJSON(t *testing.T) {
	db := openCMSDB(t)
	p := services.NewPageService(db)
	_, err := p.Upsert("home", services.PageInput{
		Title: "Home", BodyJSON: `not-json`,
	})
	if err == nil || !strings.Contains(err.Error(), "validation") {
		t.Fatalf("want validation error, got %v", err)
	}
	page, err := p.Upsert("home", services.PageInput{
		Title:    "Home",
		BodyJSON: `{"eyebrow":"Hi","headline":"H","intro":"I","domains":[],"now":""}`,
	})
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if page.Slug != "home" || page.Title != "Home" {
		t.Fatalf("page = %+v", page)
	}
}

func TestWorkCRUD(t *testing.T) {
	db := openCMSDB(t)
	w := services.NewWorkService(db)
	item, err := w.Create(services.WorkInput{
		Name: "demo", OneLiner: "x", Stack: []string{"Go"}, SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	list, err := w.List()
	if err != nil || len(list) != 1 {
		t.Fatalf("list: %v len=%d", err, len(list))
	}
	if len(list[0].Stack) != 1 || list[0].Stack[0] != "Go" {
		t.Fatalf("stack = %+v", list[0].Stack)
	}
	if err := w.Delete(item.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
}

func TestStudioRequiresMediaWhenSet(t *testing.T) {
	db := openCMSDB(t)
	uploads := filepath.Join(t.TempDir(), "up")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media: %v", err)
	}
	studio := services.NewStudioService(db)
	badID := int64(999)
	_, err = studio.Create(services.StudioInput{
		Slug: "a", Title: "A", ImageMediaID: &badID, Published: true,
	})
	if err == nil {
		t.Fatal("expected validation for missing media")
	}

	png := []byte("\x89PNG\r\n\x1a\n")
	asset, err := media.Create("still.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	piece, err := studio.Create(services.StudioInput{
		Slug: "a", Title: "A", ImageMediaID: &asset.ID, Published: true,
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if piece.ImageURL != "/media/"+strconv.FormatInt(asset.ID, 10) {
		t.Fatalf("image_url = %q", piece.ImageURL)
	}
	pub, err := studio.ListPublished()
	if err != nil || len(pub) != 1 {
		t.Fatalf("published: %v len=%d", err, len(pub))
	}
}

func TestResumeSectionCRUD(t *testing.T) {
	db := openCMSDB(t)
	r := services.NewResumeService(db)
	sec, err := r.CreateSection(services.ResumeSectionInput{
		Kind: models.ResumeKindExperience, Title: "Exp", SortOrder: 10, Accordion: true,
	})
	if err != nil {
		t.Fatalf("create section: %v", err)
	}
	if !sec.Accordion {
		t.Fatalf("expected accordion true, got %+v", sec)
	}
	sec, err = r.UpdateSection(sec.ID, services.ResumeSectionInput{
		Kind: sec.Kind, Title: sec.Title, SortOrder: sec.SortOrder, Accordion: false,
	})
	if err != nil {
		t.Fatalf("update section: %v", err)
	}
	if sec.Accordion {
		t.Fatalf("expected accordion false after update, got %+v", sec)
	}
	_, err = r.CreateEntry(services.ResumeEntryInput{
		SectionID: sec.ID, Org: "Org", Role: "Role", BodyMD: "hi",
	})
	if err != nil {
		t.Fatalf("create entry: %v", err)
	}
	if err := r.DeleteSection(sec.ID); err != nil {
		t.Fatalf("delete section: %v", err)
	}
	entries, err := r.AdminListEntries()
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	if len(entries) != 0 {
		// placeholders from 0004 may still exist; filter by section
		for _, e := range entries {
			if e.SectionID == sec.ID {
				t.Fatalf("entry survived section delete: %+v", e)
			}
		}
	}
}

func TestContentImport(t *testing.T) {
	db := openCMSDB(t)
	imp := &services.ImportService{
		DB:       db,
		Settings: services.NewSettingsService(db),
		Pages:    services.NewPageService(db),
		Work:     services.NewWorkService(db),
		Studio:   services.NewStudioService(db),
		Resume:   services.NewResumeService(db),
	}
	result, err := imp.Apply(services.ContentImport{
		ReplaceResume: true,
		ReplaceWork:   true,
		Settings: map[string]string{
			"site_name": "Imported",
		},
		Pages: []services.PageImport{{
			Slug: "home", Title: "Home",
			BodyJSON: `{"eyebrow":"","headline":"H","intro":"","domains":[],"now":""}`,
		}},
		Work: []services.WorkInput{{Name: "w1", SortOrder: 1}},
		Sections: []services.ResumeSectionInput{{
			Kind: models.ResumeKindEducation, Title: "Edu", SortOrder: 1,
		}},
		Entries: []services.ResumeEntryInput{{
			SectionID: 1, Org: "Uni", Role: "BSc",
		}},
	})
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.PagesUpserted != 1 || result.WorkCreated != 1 || result.SectionsCreated != 1 || result.EntriesCreated != 1 {
		t.Fatalf("result = %+v", result)
	}
	pub, err := imp.Settings.Public()
	if err != nil || pub.SiteName != "Imported" {
		t.Fatalf("settings: %v %+v", err, pub)
	}
}

func TestContentImportFromJSONSnakeCase(t *testing.T) {
	db := openCMSDB(t)
	imp := &services.ImportService{
		DB:       db,
		Settings: services.NewSettingsService(db),
		Pages:    services.NewPageService(db),
		Work:     services.NewWorkService(db),
		Studio:   services.NewStudioService(db),
		Resume:   services.NewResumeService(db),
	}
	raw := `{
		"replace_resume": true,
		"resume_sections": [{"kind":"experience","title":"Experience","sort_order":1}],
		"resume_entries": [{"section_id":1,"org":"Acme","role":"Eng","body_md":"did things","sort_order":1}],
		"work": [{"name":"tool","one_liner":"does a thing","stack":["Go"],"sort_order":1}]
	}`
	var dump services.ContentImport
	if err := json.Unmarshal([]byte(raw), &dump); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	result, err := imp.Apply(dump)
	if err != nil {
		t.Fatalf("import: %v", err)
	}
	if result.SectionsCreated != 1 || result.EntriesCreated != 1 || result.WorkCreated != 1 {
		t.Fatalf("result = %+v", result)
	}
	entries, err := imp.Resume.AdminListEntries()
	if err != nil {
		t.Fatalf("entries: %v", err)
	}
	found := false
	for _, e := range entries {
		if e.Org == "Acme" && e.SectionID > 0 {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected Acme entry with real section id, got %+v", entries)
	}
}

func TestContentExportRoundTrip(t *testing.T) {
	db := openCMSDB(t)
	imp := &services.ImportService{
		DB:       db,
		Settings: services.NewSettingsService(db),
		Pages:    services.NewPageService(db),
		Work:     services.NewWorkService(db),
		Studio:   services.NewStudioService(db),
		Resume:   services.NewResumeService(db),
	}
	_, err := imp.Apply(services.ContentImport{
		ReplaceResume: true,
		ReplaceWork:   true,
		Settings:      map[string]string{"site_name": "Export Me"},
		Pages: []services.PageImport{{
			Slug: "home", Title: "Home",
			BodyJSON: `{"eyebrow":"","headline":"Hi","intro":"","domains":[],"now":"","accordion":false}`,
		}},
		Work: []services.WorkInput{{Name: "proj", OneLiner: "x", SortOrder: 1}},
		Sections: []services.ResumeSectionInput{{
			Kind: models.ResumeKindExperience, Title: "Experience", SortOrder: 1,
		}},
		Entries: []services.ResumeEntryInput{{
			SectionID: 1, Org: "Co", Role: "Dev", BodyMD: "shipped",
		}},
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}

	dump, err := imp.Export()
	if err != nil {
		t.Fatalf("export: %v", err)
	}
	if dump.Settings["site_name"] != "Export Me" {
		t.Fatalf("settings = %+v", dump.Settings)
	}
	if len(dump.Pages) != 1 || dump.Pages[0].Slug != "home" {
		t.Fatalf("pages = %+v", dump.Pages)
	}
	if len(dump.Work) != 1 || dump.Work[0].Name != "proj" {
		t.Fatalf("work = %+v", dump.Work)
	}
	if len(dump.Sections) != 1 || len(dump.Entries) != 1 || dump.Entries[0].SectionID != 1 {
		t.Fatalf("resume sections=%+v entries=%+v", dump.Sections, dump.Entries)
	}
	if !dump.ReplaceResume || !dump.ReplaceWork || !dump.ReplaceStudio {
		t.Fatalf("replace flags not set: %+v", dump)
	}

	// Round-trip onto a fresh DB shape by re-applying export.
	_, err = imp.Apply(dump)
	if err != nil {
		t.Fatalf("re-import: %v", err)
	}
	pub, err := imp.Settings.Public()
	if err != nil || pub.SiteName != "Export Me" {
		t.Fatalf("after re-import: %v %+v", err, pub)
	}
}

func TestImportRollbackAfterReplaceWipe(t *testing.T) {
	db := openCMSDB(t)
	work := services.NewWorkService(db)
	_, err := work.Create(services.WorkInput{Name: "keep-me", SortOrder: 1})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	imp := &services.ImportService{
		DB:       db,
		Settings: services.NewSettingsService(db),
		Pages:    services.NewPageService(db),
		Work:     work,
		Studio:   services.NewStudioService(db),
		Resume:   services.NewResumeService(db),
	}
	_, err = imp.Apply(services.ContentImport{
		ReplaceWork: true,
		Work: []services.WorkInput{{
			Name: "evil", Href: "javascript:alert(1)", SortOrder: 1,
		}},
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	list, err := work.List()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Name != "keep-me" {
		t.Fatalf("expected keep-me preserved, got %+v", list)
	}
}

func TestImportRollbackMidCreate(t *testing.T) {
	db := openCMSDB(t)
	work := services.NewWorkService(db)
	_, err := work.Create(services.WorkInput{Name: "seed-a", SortOrder: 1})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	before, err := work.List()
	if err != nil {
		t.Fatalf("list before: %v", err)
	}
	imp := &services.ImportService{
		DB:       db,
		Settings: services.NewSettingsService(db),
		Pages:    services.NewPageService(db),
		Work:     work,
		Studio:   services.NewStudioService(db),
		Resume:   services.NewResumeService(db),
	}
	_, err = imp.Apply(services.ContentImport{
		ReplaceWork: true,
		Work: []services.WorkInput{
			{Name: "valid-new", SortOrder: 1},
			{Name: "bad", Href: "javascript:alert(1)", SortOrder: 2},
		},
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	after, err := work.List()
	if err != nil {
		t.Fatalf("list after: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("len after=%d before=%d; after=%+v", len(after), len(before), after)
	}
	for i := range before {
		if after[i].ID != before[i].ID || after[i].Name != before[i].Name {
			t.Fatalf("row %d changed: before=%+v after=%+v", i, before[i], after[i])
		}
	}
	for _, w := range after {
		if w.Name == "valid-new" {
			t.Fatalf("valid create was committed: %+v", after)
		}
	}
}

func TestImportStudioRollbackAfterReplaceWipe(t *testing.T) {
	db := openCMSDB(t)
	studio := services.NewStudioService(db)
	_, err := studio.Create(services.StudioInput{
		Slug: "keep-piece", Title: "Keep Piece", SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed: %v", err)
	}
	imp := &services.ImportService{
		DB:       db,
		Settings: services.NewSettingsService(db),
		Pages:    services.NewPageService(db),
		Work:     services.NewWorkService(db),
		Studio:   studio,
		Resume:   services.NewResumeService(db),
	}
	badID := int64(99999)
	_, err = imp.Apply(services.ContentImport{
		ReplaceStudio: true,
		Studio: []services.StudioInput{
			{Slug: "ok-new", Title: "Ok New", SortOrder: 1},
			{Slug: "bad", Title: "Bad", ImageMediaID: &badID, SortOrder: 2},
		},
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}
	list, err := studio.AdminList()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(list) != 1 || list[0].Slug != "keep-piece" {
		t.Fatalf("expected keep-piece preserved, got %+v", list)
	}
	for _, p := range list {
		if p.Slug == "ok-new" {
			t.Fatalf("valid studio create was committed: %+v", list)
		}
	}
}

func TestImportCrossDomainRollback(t *testing.T) {
	db := openCMSDB(t)
	settings := services.NewSettingsService(db)
	resume := services.NewResumeService(db)
	_, err := settings.Upsert(map[string]string{"site_name": "Original"})
	if err != nil {
		t.Fatalf("seed settings: %v", err)
	}
	sec, err := resume.CreateSection(services.ResumeSectionInput{
		Kind: models.ResumeKindExperience, Title: "Keep Section", SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("seed section: %v", err)
	}
	_, err = resume.CreateEntry(services.ResumeEntryInput{
		SectionID: sec.ID, Org: "Keep Org", Role: "Keep Role",
	})
	if err != nil {
		t.Fatalf("seed entry: %v", err)
	}
	sectionsBefore, err := resume.ListSections()
	if err != nil {
		t.Fatalf("sections before: %v", err)
	}
	entriesBefore, err := resume.AdminListEntries()
	if err != nil {
		t.Fatalf("entries before: %v", err)
	}

	imp := &services.ImportService{
		DB:       db,
		Settings: settings,
		Pages:    services.NewPageService(db),
		Work:     services.NewWorkService(db),
		Studio:   services.NewStudioService(db),
		Resume:   resume,
	}
	_, err = imp.Apply(services.ContentImport{
		ReplaceResume: true,
		Settings:      map[string]string{"site_name": "Should Not Stick"},
		Sections: []services.ResumeSectionInput{{
			Kind: models.ResumeKindEducation, Title: "Imported Edu", SortOrder: 1,
		}},
		Entries: []services.ResumeEntryInput{{
			SectionID: 99, Org: "Bad", Role: "Bad",
		}},
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("want ErrValidation, got %v", err)
	}

	pub, err := settings.Public()
	if err != nil || pub.SiteName != "Original" {
		t.Fatalf("settings changed: %v %+v", err, pub)
	}
	sectionsAfter, err := resume.ListSections()
	if err != nil {
		t.Fatalf("sections after: %v", err)
	}
	if len(sectionsAfter) != len(sectionsBefore) {
		t.Fatalf("sections len after=%d before=%d", len(sectionsAfter), len(sectionsBefore))
	}
	foundKeep := false
	for _, s := range sectionsAfter {
		if s.ID == sec.ID && s.Title == "Keep Section" {
			foundKeep = true
		}
		if s.Title == "Imported Edu" {
			t.Fatalf("imported section committed: %+v", sectionsAfter)
		}
	}
	if !foundKeep {
		t.Fatalf("keep section missing: %+v", sectionsAfter)
	}
	entriesAfter, err := resume.AdminListEntries()
	if err != nil {
		t.Fatalf("entries after: %v", err)
	}
	if len(entriesAfter) != len(entriesBefore) {
		t.Fatalf("entries len after=%d before=%d", len(entriesAfter), len(entriesBefore))
	}
	foundOrg := false
	for _, e := range entriesAfter {
		if e.Org == "Keep Org" {
			foundOrg = true
		}
		if e.Org == "Bad" {
			t.Fatalf("bad entry committed: %+v", entriesAfter)
		}
	}
	if !foundOrg {
		t.Fatalf("keep entry missing: %+v", entriesAfter)
	}
}

func TestURLAllowlistOnWrites(t *testing.T) {
	db := openCMSDB(t)
	settings := services.NewSettingsService(db)
	pages := services.NewPageService(db)
	work := services.NewWorkService(db)
	imp := &services.ImportService{
		DB:       db,
		Settings: settings,
		Pages:    pages,
		Work:     work,
		Studio:   services.NewStudioService(db),
		Resume:   services.NewResumeService(db),
	}

	_, err := work.Create(services.WorkInput{
		Name: "ok", Href: "https://example.com/x", SortOrder: 1,
	})
	if err != nil {
		t.Fatalf("good href: %v", err)
	}
	_, err = work.Create(services.WorkInput{Name: "empty-href", SortOrder: 2})
	if err != nil {
		t.Fatalf("empty href: %v", err)
	}
	for _, bad := range []string{"javascript:alert(1)", "//evil.com", "data:text/html,x", "vbscript:msg"} {
		_, err = work.Create(services.WorkInput{Name: "bad", Href: bad})
		if err == nil || !errors.Is(err, services.ErrValidation) {
			t.Fatalf("href %q: want ErrValidation, got %v", bad, err)
		}
	}

	_, err = settings.Upsert(map[string]string{
		"nav": `[{"label":"Home","path":"/"},{"label":"Work","path":"/work"}]`,
	})
	if err != nil {
		t.Fatalf("good nav: %v", err)
	}
	_, err = settings.Upsert(map[string]string{
		"nav": `[{"label":"X","path":"//evil.com"}]`,
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("bad nav path: want ErrValidation, got %v", err)
	}
	_, err = settings.Upsert(map[string]string{"nav": `not-json`})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("bad nav json: want ErrValidation, got %v", err)
	}

	_, err = settings.Upsert(map[string]string{
		"contact": `{"email":"a@b.c","github":"https://github.com/x","linkedin":""}`,
	})
	if err != nil {
		t.Fatalf("good contact: %v", err)
	}
	_, err = settings.Upsert(map[string]string{
		"contact": `{"email":"bad","github":"","linkedin":""}`,
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("bad email: want ErrValidation, got %v", err)
	}
	_, err = settings.Upsert(map[string]string{
		"contact": `{"email":"","github":"javascript:alert(1)","linkedin":""}`,
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("bad github: want ErrValidation, got %v", err)
	}

	_, err = pages.Upsert("home", services.PageInput{
		Title:    "Home",
		BodyJSON: `{"eyebrow":"","headline":"H","intro":"","domains":[{"title":"A","blurb":"b","offset":"","link":{"to":"/work","label":"Work"}}],"now":""}`,
	})
	if err != nil {
		t.Fatalf("good home link: %v", err)
	}
	_, err = pages.Upsert("home", services.PageInput{
		Title:    "Home",
		BodyJSON: `{"eyebrow":"","headline":"H","intro":"","domains":[{"title":"A","blurb":"b","offset":"","link":{"to":"javascript:alert(1)","label":"X"}}],"now":""}`,
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("bad domain link: want ErrValidation, got %v", err)
	}
	_, err = pages.Upsert("home", services.PageInput{
		Title:    "Home",
		BodyJSON: `{"eyebrow":"","headline":"H","intro":"","domains":[{"title":"A","blurb":"b","offset":"","link":{"to":"","label":"X"}}],"now":""}`,
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("empty link.to: want ErrValidation, got %v", err)
	}

	_, err = imp.Apply(services.ContentImport{
		Work: []services.WorkInput{{Name: "evil", Href: "javascript:alert(1)"}},
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("import bad href: want ErrValidation, got %v", err)
	}
}

func TestIsPubliclyReferenced(t *testing.T) {
	db := openCMSDB(t)
	uploads := filepath.Join(t.TempDir(), "up")
	media, err := services.NewMediaService(db, uploads)
	if err != nil {
		t.Fatalf("media: %v", err)
	}
	studio := services.NewStudioService(db)
	pages := services.NewPageService(db)
	posts := services.NewPostService(db)

	png := []byte("\x89PNG\r\n\x1a\n")
	orphan, err := media.Create("orphan.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("orphan: %v", err)
	}
	ok, err := media.IsPubliclyReferenced(orphan.ID)
	if err != nil || ok {
		t.Fatalf("orphan public = %v, err=%v, want false", ok, err)
	}

	draftAsset, err := media.Create("draft.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("draft asset: %v", err)
	}
	_, err = studio.Create(services.StudioInput{
		Slug: "draft-piece", Title: "Draft", ImageMediaID: &draftAsset.ID, Published: false,
	})
	if err != nil {
		t.Fatalf("draft studio: %v", err)
	}
	ok, err = media.IsPubliclyReferenced(draftAsset.ID)
	if err != nil || ok {
		t.Fatalf("unpublished studio public = %v, err=%v, want false", ok, err)
	}

	pubAsset, err := media.Create("pub.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("pub asset: %v", err)
	}
	_, err = studio.Create(services.StudioInput{
		Slug: "pub-piece", Title: "Pub", ImageMediaID: &pubAsset.ID, Published: true,
	})
	if err != nil {
		t.Fatalf("pub studio: %v", err)
	}
	ok, err = media.IsPubliclyReferenced(pubAsset.ID)
	if err != nil || !ok {
		t.Fatalf("published studio public = %v, err=%v, want true", ok, err)
	}

	pdf := []byte("%PDF-1.4")
	resumePDF, err := media.Create("cv.pdf", "application/pdf", bytes.NewReader(pdf), int64(len(pdf)))
	if err != nil {
		t.Fatalf("resume pdf: %v", err)
	}
	body, err := json.Marshal(map[string]any{
		"eyebrow": "CV", "headline": "Resume", "blurb": "Hi", "pdf_media_id": resumePDF.ID,
	})
	if err != nil {
		t.Fatalf("marshal resume: %v", err)
	}
	_, err = pages.Upsert("resume", services.PageInput{
		Title: "Resume", BodyJSON: string(body),
	})
	if err != nil {
		t.Fatalf("upsert resume: %v", err)
	}
	ok, err = media.IsPubliclyReferenced(resumePDF.ID)
	if err != nil || !ok {
		t.Fatalf("resume pdf public = %v, err=%v, want true", ok, err)
	}

	// Digit-bounded: /media/10 must not make id=1 public.
	_, err = posts.Create(services.PostInput{
		Slug: "with-media", Title: "With Media", Summary: "s",
		ContentMD: "See ![x](/media/10) trailing",
		Published: true,
	})
	if err != nil {
		t.Fatalf("published post: %v", err)
	}
	ok, err = media.IsPubliclyReferenced(10)
	if err != nil || !ok {
		t.Fatalf("post-ref id=10 public = %v, err=%v, want true", ok, err)
	}
	ok, err = media.IsPubliclyReferenced(1)
	if err != nil || ok {
		t.Fatalf("post-ref id=1 via /media/10 = %v, err=%v, want false", ok, err)
	}

	_, err = posts.Create(services.PostInput{
		Slug: "draft-ref", Title: "Draft Ref", Summary: "s",
		ContentMD: "![d](/media/" + strconv.FormatInt(orphan.ID, 10) + ")",
		Published: false,
	})
	if err != nil {
		t.Fatalf("draft post: %v", err)
	}
	ok, err = media.IsPubliclyReferenced(orphan.ID)
	if err != nil || ok {
		t.Fatalf("draft post ref public = %v, err=%v, want false", ok, err)
	}

	htmlOnly, err := media.Create("html-only.png", "image/png", bytes.NewReader(png), int64(len(png)))
	if err != nil {
		t.Fatalf("html-only asset: %v", err)
	}
	htmlPost, err := posts.Create(services.PostInput{
		Slug: "html-only-ref", Title: "HTML Only", Summary: "s",
		ContentMD: "no media here",
		Published: true,
	})
	if err != nil {
		t.Fatalf("html-only post: %v", err)
	}
	_, err = db.Exec(
		`UPDATE posts SET content_html = ? WHERE id = ?`,
		`<p><img src="/media/`+strconv.FormatInt(htmlOnly.ID, 10)+`"></p>`,
		htmlPost.ID,
	)
	if err != nil {
		t.Fatalf("set content_html: %v", err)
	}
	ok, err = media.IsPubliclyReferenced(htmlOnly.ID)
	if err != nil || !ok {
		t.Fatalf("content_html-only public = %v, err=%v, want true", ok, err)
	}
}
