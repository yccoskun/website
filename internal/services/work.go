package services

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yccoskun/website/internal/models"
)

// WorkService manages work portfolio items.
type WorkService struct {
	db *sql.DB
}

// NewWorkService constructs a WorkService.
func NewWorkService(db *sql.DB) *WorkService {
	return &WorkService{db: db}
}

// WorkInput is the writable work item payload.
type WorkInput struct {
	Name      string   `json:"name"`
	OneLiner  string   `json:"one_liner"`
	Body      string   `json:"body"`
	Stack     []string `json:"stack"`
	Status    string   `json:"status"`
	Href      string   `json:"href"`
	SortOrder int      `json:"sort_order"`
}

const workColumns = `id, name, one_liner, body, stack_json, status, href, sort_order`

func scanWorkItem(scanner interface {
	Scan(dest ...any) error
}) (models.WorkItem, error) {
	var (
		w         models.WorkItem
		stackJSON string
	)
	err := scanner.Scan(
		&w.ID, &w.Name, &w.OneLiner, &w.Body, &stackJSON, &w.Status, &w.Href, &w.SortOrder,
	)
	if err != nil {
		return models.WorkItem{}, err
	}
	w.Stack = []string{}
	if stackJSON != "" {
		_ = json.Unmarshal([]byte(stackJSON), &w.Stack)
		if w.Stack == nil {
			w.Stack = []string{}
		}
	}
	return w, nil
}

func encodeStack(stack []string) (string, error) {
	if stack == nil {
		stack = []string{}
	}
	b, err := json.Marshal(stack)
	if err != nil {
		return "", fmt.Errorf("%w: stack: %v", ErrValidation, err)
	}
	return string(b), nil
}

func validateWorkInput(in WorkInput) error {
	if strings.TrimSpace(in.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrValidation)
	}
	return nil
}

// List returns all work items ordered for public display.
func (s *WorkService) List() ([]models.WorkItem, error) {
	rows, err := s.db.Query(
		`SELECT ` + workColumns + ` FROM work_items ORDER BY sort_order ASC, id ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list work items: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return collectWorkItems(rows)
}

// GetByID returns a work item.
func (s *WorkService) GetByID(id int64) (models.WorkItem, error) {
	row := s.db.QueryRow(`SELECT `+workColumns+` FROM work_items WHERE id = ?`, id)
	w, err := scanWorkItem(row)
	if errors.Is(err, sql.ErrNoRows) {
		return models.WorkItem{}, ErrNotFound
	}
	if err != nil {
		return models.WorkItem{}, fmt.Errorf("get work item: %w", err)
	}
	return w, nil
}

// Create inserts a work item.
func (s *WorkService) Create(in WorkInput) (models.WorkItem, error) {
	if err := validateWorkInput(in); err != nil {
		return models.WorkItem{}, err
	}
	stackJSON, err := encodeStack(in.Stack)
	if err != nil {
		return models.WorkItem{}, err
	}
	res, err := s.db.Exec(
		`INSERT INTO work_items (name, one_liner, body, stack_json, status, href, sort_order)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(in.Name), in.OneLiner, in.Body, stackJSON, in.Status, in.Href, in.SortOrder,
	)
	if err != nil {
		return models.WorkItem{}, fmt.Errorf("create work item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return models.WorkItem{}, fmt.Errorf("create work item id: %w", err)
	}
	return s.GetByID(id)
}

// Update replaces a work item.
func (s *WorkService) Update(id int64, in WorkInput) (models.WorkItem, error) {
	if _, err := s.GetByID(id); err != nil {
		return models.WorkItem{}, err
	}
	if err := validateWorkInput(in); err != nil {
		return models.WorkItem{}, err
	}
	stackJSON, err := encodeStack(in.Stack)
	if err != nil {
		return models.WorkItem{}, err
	}
	_, err = s.db.Exec(
		`UPDATE work_items SET name = ?, one_liner = ?, body = ?, stack_json = ?,
		 status = ?, href = ?, sort_order = ? WHERE id = ?`,
		strings.TrimSpace(in.Name), in.OneLiner, in.Body, stackJSON, in.Status, in.Href, in.SortOrder, id,
	)
	if err != nil {
		return models.WorkItem{}, fmt.Errorf("update work item: %w", err)
	}
	return s.GetByID(id)
}

// Delete removes a work item.
func (s *WorkService) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM work_items WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete work item: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("delete work item rows: %w", err)
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func collectWorkItems(rows *sql.Rows) ([]models.WorkItem, error) {
	out := make([]models.WorkItem, 0)
	for rows.Next() {
		w, err := scanWorkItem(rows)
		if err != nil {
			return nil, fmt.Errorf("scan work item: %w", err)
		}
		out = append(out, w)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate work items: %w", err)
	}
	return out, nil
}
