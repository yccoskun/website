package services_test

import (
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
