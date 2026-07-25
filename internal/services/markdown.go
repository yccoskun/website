package services

import (
	"bytes"
	"fmt"

	"github.com/microcosm-cc/bluemonday"
	"github.com/yuin/goldmark"
)

var (
	mdRenderer = goldmark.New()
	mdPolicy   = bluemonday.UGCPolicy()
)

// RenderMarkdown converts Markdown to sanitized HTML suitable for storage
// and public rendering.
func RenderMarkdown(src string) (string, error) {
	var buf bytes.Buffer
	if err := mdRenderer.Convert([]byte(src), &buf); err != nil {
		return "", fmt.Errorf("render markdown: %w", err)
	}
	return mdPolicy.Sanitize(buf.String()), nil
}
