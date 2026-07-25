package handlers

import (
	"encoding/xml"
	"fmt"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/yccoskun/website/internal/models"
)

const defaultSiteURL = "https://www.yusufcancoskun.com"

func (d Deps) siteURL() string {
	u := strings.TrimRight(d.Config.SiteURL, "/")
	if u == "" {
		return defaultSiteURL
	}
	return u
}

// Robots serves robots.txt for crawlers.
func (d Deps) Robots(w http.ResponseWriter, _ *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	_, _ = fmt.Fprintf(w,
		"User-agent: *\nAllow: /\nDisallow: /admin\nDisallow: /api/admin\nSitemap: %s/sitemap.xml\n",
		d.siteURL(),
	)
}

type sitemapURLSet struct {
	XMLName xml.Name     `xml:"urlset"`
	Xmlns   string       `xml:"xmlns,attr"`
	URLs    []sitemapURL `xml:"url"`
}

type sitemapURL struct {
	Loc     string `xml:"loc"`
	LastMod string `xml:"lastmod,omitempty"`
}

// Sitemap serves sitemap.xml with static routes and published posts.
func (d Deps) Sitemap(w http.ResponseWriter, _ *http.Request) {
	base := d.siteURL()
	urls := []sitemapURL{
		{Loc: base + "/"},
		{Loc: base + "/blog"},
		{Loc: base + "/resume"},
	}

	if d.Posts != nil {
		posts, err := d.Posts.ListPublished()
		if err != nil {
			log.Printf("sitemap: list posts: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		for _, p := range posts {
			urls = append(urls, sitemapURL{
				Loc:     base + "/blog/" + p.Slug,
				LastMod: postLastMod(p),
			})
		}
	}

	w.Header().Set("Content-Type", "application/xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(sitemapURLSet{
		Xmlns: "http://www.sitemaps.org/schemas/sitemap/0.9",
		URLs:  urls,
	}); err != nil {
		log.Printf("sitemap: encode: %v", err)
	}
}

type rssFeed struct {
	XMLName xml.Name   `xml:"rss"`
	Version string     `xml:"version,attr"`
	Channel rssChannel `xml:"channel"`
}

type rssChannel struct {
	Title       string    `xml:"title"`
	Link        string    `xml:"link"`
	Description string    `xml:"description"`
	Items       []rssItem `xml:"item"`
}

type rssItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate,omitempty"`
	GUID        string `xml:"guid"`
}

// RSS serves an RSS 2.0 feed of published posts.
func (d Deps) RSS(w http.ResponseWriter, _ *http.Request) {
	base := d.siteURL()
	items := make([]rssItem, 0)
	if d.Posts != nil {
		posts, err := d.Posts.ListPublished()
		if err != nil {
			log.Printf("rss: list posts: %v", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		for _, p := range posts {
			link := base + "/blog/" + p.Slug
			items = append(items, rssItem{
				Title:       p.Title,
				Link:        link,
				Description: p.Summary,
				PubDate:     postPubDate(p),
				GUID:        link,
			})
		}
	}

	w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
	_, _ = w.Write([]byte(xml.Header))
	enc := xml.NewEncoder(w)
	enc.Indent("", "  ")
	if err := enc.Encode(rssFeed{
		Version: "2.0",
		Channel: rssChannel{
			Title:       "Yusuf Can Coskun",
			Link:        base + "/",
			Description: "Blog of Yusuf Can Coskun.",
			Items:       items,
		},
	}); err != nil {
		log.Printf("rss: encode: %v", err)
	}
}

func postLastMod(p models.PostSummary) string {
	if p.PublishedAt != nil && *p.PublishedAt != "" {
		return truncateDate(*p.PublishedAt)
	}
	return truncateDate(p.UpdatedAt)
}

func postPubDate(p models.PostSummary) string {
	raw := p.UpdatedAt
	if p.PublishedAt != nil && *p.PublishedAt != "" {
		raw = *p.PublishedAt
	}
	t, err := parseStoredTime(raw)
	if err != nil {
		return ""
	}
	return t.UTC().Format(time.RFC1123Z)
}

func truncateDate(s string) string {
	if len(s) >= 10 {
		return s[:10]
	}
	return s
}

func parseStoredTime(s string) (time.Time, error) {
	layouts := []string{
		"2006-01-02T15:04:05.000Z",
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("unrecognized time %q", s)
}
