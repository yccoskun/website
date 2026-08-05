package services_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yccoskun/website/internal/services"
)

func TestPostSlugValidation(t *testing.T) {
	db := openCMSDB(t)
	posts := services.NewPostService(db)

	accept := []string{"hello-world", "a1", "post", "a", strings.Repeat("a", 100)}
	for _, slug := range accept {
		_, err := posts.Create(services.PostInput{
			Slug: slug, Title: "Title", Summary: "s", ContentMD: "# hi",
		})
		if err != nil {
			t.Fatalf("slug %q: unexpected error: %v", slug, err)
		}
	}

	reject := []struct {
		name string
		slug string
	}{
		{name: "empty", slug: ""},
		{name: "whitespace", slug: "   "},
		{name: "spaces", slug: "hello world"},
		{name: "cafe", slug: "café"},
		{name: "cyrillic", slug: "привет"},
		{name: "path traversal", slug: "../etc"},
		{name: "uppercase", slug: "Hello"},
		{name: "double hyphen lead", slug: "--x"},
		{name: "double hyphen trail", slug: "x--"},
		{name: "leading hyphen", slug: "-x"},
		{name: "trailing hyphen", slug: "x-"},
		{name: "double hyphen mid", slug: "a--b"},
		{name: "underscore", slug: "hello_world"},
		{name: "dot", slug: "hello.world"},
		{name: "too long", slug: strings.Repeat("a", 101)},
	}
	for _, tc := range reject {
		t.Run(tc.name, func(t *testing.T) {
			_, err := posts.Create(services.PostInput{
				Slug: tc.slug, Title: "Title", Summary: "s", ContentMD: "# hi",
			})
			if err == nil || !errors.Is(err, services.ErrValidation) {
				t.Fatalf("slug %q: want ErrValidation, got %v", tc.slug, err)
			}
		})
	}
}

func TestPostCreateThenUpdateSameSlug(t *testing.T) {
	db := openCMSDB(t)
	posts := services.NewPostService(db)

	created, err := posts.Create(services.PostInput{
		Slug: "good-slug", Title: "First", Summary: "s", ContentMD: "# one",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	updated, err := posts.Update(created.ID, services.PostInput{
		Slug: "good-slug", Title: "Second", Summary: "s2", ContentMD: "# two", Published: true,
	})
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if updated.Slug != "good-slug" || updated.Title != "Second" || !updated.Published {
		t.Fatalf("unexpected update result: %+v", updated)
	}
}

func TestPostUpdateRejectsBadSlug(t *testing.T) {
	db := openCMSDB(t)
	posts := services.NewPostService(db)

	created, err := posts.Create(services.PostInput{
		Slug: "ok-slug", Title: "First", Summary: "s", ContentMD: "# one",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	_, err = posts.Update(created.ID, services.PostInput{
		Slug: "bad slug", Title: "Second", Summary: "s2", ContentMD: "# two",
	})
	if err == nil || !errors.Is(err, services.ErrValidation) {
		t.Fatalf("want ErrValidation on bad update slug, got %v", err)
	}
}

func TestAdminListOmitsContentBodies(t *testing.T) {
	db := openCMSDB(t)
	posts := services.NewPostService(db)

	const bodyMD = "## Non-trivial draft body\n\nParagraph with **markdown** and a list:\n\n- one\n- two\n"
	_, err := posts.Create(services.PostInput{
		Slug: "older-post", Title: "Older", Summary: "older summary",
		ContentMD: bodyMD, Published: false,
	})
	if err != nil {
		t.Fatalf("create draft: %v", err)
	}

	newer, err := posts.Create(services.PostInput{
		Slug: "newer-post", Title: "Newer", Summary: "newer summary",
		ContentMD: "# Published body with enough content to render HTML", Published: true,
	})
	if err != nil {
		t.Fatalf("create published: %v", err)
	}

	list, err := posts.AdminList()
	if err != nil {
		t.Fatalf("AdminList: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("AdminList len = %d, want 2", len(list))
	}
	if list[0].Slug != "newer-post" || list[1].Slug != "older-post" {
		t.Fatalf("order = [%s, %s], want [newer-post, older-post]", list[0].Slug, list[1].Slug)
	}
	if list[0].Published != true || list[1].Published != false {
		t.Fatalf("published flags = [%v, %v], want [true, false]", list[0].Published, list[1].Published)
	}

	raw, err := json.Marshal(list)
	if err != nil {
		t.Fatalf("marshal list: %v", err)
	}
	var items []map[string]json.RawMessage
	if err := json.Unmarshal(raw, &items); err != nil {
		t.Fatalf("unmarshal list maps: %v", err)
	}
	for i, item := range items {
		for _, forbidden := range []string{"content_md", "content_html"} {
			if _, ok := item[forbidden]; ok {
				t.Fatalf("item[%d] has forbidden key %q: %s", i, forbidden, raw)
			}
		}
		for _, required := range []string{"published", "title", "slug", "created_at", "updated_at"} {
			if _, ok := item[required]; !ok {
				t.Fatalf("item[%d] missing %q: %s", i, required, raw)
			}
		}
	}

	got, err := posts.GetByID(newer.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got.ContentMD == "" || got.ContentHTML == "" {
		t.Fatalf("GetByID bodies empty: md=%q html=%q", got.ContentMD, got.ContentHTML)
	}
	if got.ContentMD != "# Published body with enough content to render HTML" {
		t.Fatalf("GetByID content_md = %q", got.ContentMD)
	}
}
