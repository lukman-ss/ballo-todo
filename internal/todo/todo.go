package todo

import (
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

var ErrNotFound = errors.New("todo not found")

type Todo struct {
	ID        int64
	Title     string
	Notes     string
	Completed bool
	Priority  string
	DueDate   string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ListFilter struct {
	Status  string // "all", "active", "completed"
	Search  string // searches title and notes
	Sort    string // "newest", "due", "priority"
	Page    int    // 1-based
	PerPage int    // default 20
}

type UpdateReq struct {
	Title     *string
	Notes     *string
	Priority  *string
	DueDate   **string // outer ptr = field present, inner = nil means clear due date
	Completed *bool
}

type Store struct {
	db  *sql.DB
	loc *time.Location
}

func NewStore(db *sql.DB, loc *time.Location) *Store {
	return &Store{db: db, loc: loc}
}

func validate(title, notes, priority, dueDate string) error {
	title = strings.TrimSpace(title)
	if title == "" {
		return fmt.Errorf("title is required")
	}
	if len(title) > 200 {
		return fmt.Errorf("title must be 200 chars or fewer")
	}
	if len(notes) > 5000 {
		return fmt.Errorf("notes must be 5000 chars or fewer")
	}
	switch priority {
	case "low", "medium", "high":
	default:
		return fmt.Errorf("priority must be low, medium, or high")
	}
	if dueDate != "" {
		if _, err := time.Parse("2006-01-02", dueDate); err != nil {
			return fmt.Errorf("due_date must be YYYY-MM-DD or empty")
		}
	}
	return nil
}

func (s *Store) Create(title, notes, priority, dueDate string) (*Todo, error) {
	title = strings.TrimSpace(title)
	if err := validate(title, notes, priority, dueDate); err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	res, err := s.db.Exec(
		`INSERT INTO todos (title, notes, priority, due_date, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`,
		title, notes, priority, dueDate, now.Format(time.RFC3339), now.Format(time.RFC3339),
	)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *Store) GetByID(id int64) (*Todo, error) {
	row := s.db.QueryRow(
		`SELECT id, title, notes, completed, priority, due_date, created_at, updated_at FROM todos WHERE id = ?`, id,
	)
	return scanTodo(row)
}

func scanTodo(row *sql.Row) (*Todo, error) {
	var t Todo
	var completed int
	var createdAt, updatedAt string
	err := row.Scan(&t.ID, &t.Title, &t.Notes, &completed, &t.Priority, &t.DueDate, &createdAt, &updatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	t.Completed = completed == 1
	t.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}
	t.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		return nil, fmt.Errorf("invalid updated_at timestamp: %w", err)
	}
	return &t, nil
}

func scanTodoFromRows(rows *sql.Rows) (*Todo, error) {
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return nil, err
		}
		return nil, sql.ErrNoRows
	}

	var t Todo
	var completed int
	var createdAt, updatedAt string
	err := rows.Scan(&t.ID, &t.Title, &t.Notes, &completed, &t.Priority, &t.DueDate, &createdAt, &updatedAt)
	if err != nil {
		rows.Close()
		return nil, err
	}
	t.Completed = completed == 1
	t.CreatedAt, err = time.Parse(time.RFC3339, createdAt)
	if err != nil {
		rows.Close()
		return nil, fmt.Errorf("invalid created_at timestamp: %w", err)
	}
	t.UpdatedAt, err = time.Parse(time.RFC3339, updatedAt)
	if err != nil {
		rows.Close()
		return nil, fmt.Errorf("invalid updated_at timestamp: %w", err)
	}
	return &t, nil
}

func (s *Store) List(filter ListFilter) ([]*Todo, error) {
	if filter.PerPage <= 0 {
		filter.PerPage = 20
	}
	if filter.Page <= 0 {
		filter.Page = 1
	}

	var conditions []string
	var args []interface{}

	switch filter.Status {
	case "active":
		conditions = append(conditions, "completed = 0")
	case "completed":
		conditions = append(conditions, "completed = 1")
	}

	if filter.Search != "" {
		conditions = append(conditions, "(title LIKE ? OR notes LIKE ?)")
		like := "%" + filter.Search + "%"
		args = append(args, like, like)
	}

	where := ""
	if len(conditions) > 0 {
		where = "WHERE " + strings.Join(conditions, " AND ")
	}

	var orderBy string
	switch filter.Sort {
	case "due":
		orderBy = `ORDER BY
			CASE WHEN due_date = '' THEN 1 ELSE 0 END ASC,
			due_date ASC,
			CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 END ASC,
			id DESC`
	case "priority":
		orderBy = `ORDER BY
			CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 END ASC,
			created_at DESC,
			id DESC`
	default:
		orderBy = `ORDER BY created_at DESC, id DESC`
	}

	offset := (filter.Page - 1) * filter.PerPage
	query := fmt.Sprintf(
		`SELECT id, title, notes, completed, priority, due_date, created_at, updated_at FROM todos %s %s LIMIT ? OFFSET ?`,
		where, orderBy,
	)
	args = append(args, filter.PerPage, offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var todos []*Todo
	for {
		todo, err := scanTodoFromRows(rows)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				break
			}
			return nil, err
		}
		todos = append(todos, todo)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return todos, nil
}

func (s *Store) Update(id int64, req UpdateReq) (*Todo, error) {
	// Fetch existing to validate and merge
	existing, err := s.GetByID(id)
	if err != nil {
		return nil, err
	}

	title := existing.Title
	notes := existing.Notes
	priority := existing.Priority
	dueDate := existing.DueDate
	completed := existing.Completed

	if req.Title != nil {
		title = strings.TrimSpace(*req.Title)
	}
	if req.Notes != nil {
		notes = *req.Notes
	}
	if req.Priority != nil {
		priority = *req.Priority
	}
	if req.DueDate != nil {
		if *req.DueDate == nil {
			dueDate = ""
		} else {
			dueDate = **req.DueDate
		}
	}
	if req.Completed != nil {
		completed = *req.Completed
	}

	if err := validate(title, notes, priority, dueDate); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	completedInt := 0
	if completed {
		completedInt = 1
	}
	_, err = s.db.Exec(
		`UPDATE todos SET title=?, notes=?, priority=?, due_date=?, completed=?, updated_at=? WHERE id=?`,
		title, notes, priority, dueDate, completedInt, now.Format(time.RFC3339), id,
	)
	if err != nil {
		return nil, err
	}
	return s.GetByID(id)
}

func (s *Store) Delete(id int64) error {
	res, err := s.db.Exec(`DELETE FROM todos WHERE id = ?`, id)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *Store) Summary() (active, completed, overdue int, err error) {
	today := time.Now().In(s.loc).Format("2006-01-02")

	row := s.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE completed = 0`)
	if err = row.Scan(&active); err != nil {
		return
	}

	row = s.db.QueryRow(`SELECT COUNT(*) FROM todos WHERE completed = 1`)
	if err = row.Scan(&completed); err != nil {
		return
	}

	row = s.db.QueryRow(
		`SELECT COUNT(*) FROM todos WHERE completed = 0 AND due_date != '' AND due_date < ?`, today,
	)
	err = row.Scan(&overdue)
	return
}
