package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/yccoskun/website/internal/models"
)

// Setting keys used across the site.
const (
	SettingSiteName        = "site_name"
	SettingMetaDescription = "meta_description"
	SettingRSSTitle        = "rss_title"
	SettingRSSDescription  = "rss_description"
	SettingMonogram        = "monogram"
	SettingNav             = "nav"
	SettingContact         = "contact"
)

// SettingsService manages site_settings key/value rows.
type SettingsService struct {
	db *sql.DB
}

// NewSettingsService constructs a SettingsService.
func NewSettingsService(db *sql.DB) *SettingsService {
	return &SettingsService{db: db}
}

// GetAll returns every setting as a key→value map (admin).
func (s *SettingsService) GetAll() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM site_settings ORDER BY key ASC`)
	if err != nil {
		return nil, fmt.Errorf("list settings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	out := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, fmt.Errorf("scan setting: %w", err)
		}
		out[k] = v
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate settings: %w", err)
	}
	return out, nil
}

// Upsert replaces or inserts settings by key.
func (s *SettingsService) Upsert(in map[string]string) (map[string]string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin settings upsert: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	stmt, err := tx.Prepare(
		`INSERT INTO site_settings (key, value) VALUES (?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value`,
	)
	if err != nil {
		return nil, fmt.Errorf("prepare settings upsert: %w", err)
	}
	defer func() { _ = stmt.Close() }()

	for k, v := range in {
		key := strings.TrimSpace(k)
		if key == "" {
			return nil, fmt.Errorf("%w: setting key is required", ErrValidation)
		}
		if err := validateSettingValue(key, v); err != nil {
			return nil, err
		}
		if _, err := stmt.Exec(key, v); err != nil {
			return nil, fmt.Errorf("upsert setting %q: %w", key, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit settings upsert: %w", err)
	}
	return s.GetAll()
}

// Public returns the public-safe settings payload with defaults when empty.
func (s *SettingsService) Public() (models.PublicSettings, error) {
	all, err := s.GetAll()
	if err != nil {
		return models.PublicSettings{}, err
	}
	return parsePublicSettings(all), nil
}

func parsePublicSettings(all map[string]string) models.PublicSettings {
	out := models.PublicSettings{
		SiteName:        all[SettingSiteName],
		MetaDescription: all[SettingMetaDescription],
		RSSTitle:        all[SettingRSSTitle],
		RSSDescription:  all[SettingRSSDescription],
		Monogram:        all[SettingMonogram],
		Nav:             []models.NavItem{},
		Contact:         models.Contact{},
	}
	if raw := all[SettingNav]; raw != "" {
		var nav []models.NavItem
		if err := json.Unmarshal([]byte(raw), &nav); err == nil && nav != nil {
			out.Nav = nav
		}
	}
	if raw := all[SettingContact]; raw != "" {
		var c models.Contact
		if err := json.Unmarshal([]byte(raw), &c); err == nil {
			out.Contact = c
		}
	}
	return out
}

func validateSettingValue(key, value string) error {
	switch key {
	case SettingNav:
		var nav []models.NavItem
		if err := json.Unmarshal([]byte(value), &nav); err != nil {
			return fmt.Errorf("%w: nav must be valid JSON", ErrValidation)
		}
		for _, item := range nav {
			if err := ValidateNavPath(item.Path); err != nil {
				return err
			}
		}
	case SettingContact:
		var c models.Contact
		if err := json.Unmarshal([]byte(value), &c); err != nil {
			return fmt.Errorf("%w: contact must be valid JSON", ErrValidation)
		}
		if err := ValidateEmail(c.Email); err != nil {
			return err
		}
		if err := ValidateHTTPSURL(c.GitHub, true); err != nil {
			return err
		}
		if err := ValidateHTTPSURL(c.LinkedIn, true); err != nil {
			return err
		}
	}
	return nil
}

// Get returns a single setting value (empty string if missing).
func (s *SettingsService) Get(key string) (string, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM site_settings WHERE key = ?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("get setting: %w", err)
	}
	return v, nil
}
