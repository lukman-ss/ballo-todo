package todo

import (
	"database/sql"
	"database/sql/driver"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
)

func TestScanTodo_invalidTimestamp(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	// Insert row with invalid timestamp
	if _, err := db.Exec(`INSERT INTO todos (title, notes, completed, priority, due_date, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"Test", "", 0, "medium", "", "not-a-timestamp", "2026-11-04T07:40:00Z"); err != nil {
		t.Fatal(err)
	}
	row := db.QueryRow(`SELECT id, title, notes, completed, priority, due_date, created_at, updated_at FROM todos WHERE id = 1`)
	_, err = scanTodo(row)
	if err == nil {
		t.Fatal("expected error for invalid created_at timestamp")
	}
	if err.Error() != "invalid created_at timestamp: parsing time \"not-a-timestamp\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"not-a-timestamp\" as \"2006\"" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func TestScanTodoFromRows_invalidTimestamp(t *testing.T) {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	// Insert row with invalid timestamp
	if _, err := db.Exec(`INSERT INTO todos (title, notes, completed, priority, due_date, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		"Test", "", 0, "medium", "", "2026-11-04T07:40:00Z", "not-a-timestamp"); err != nil {
		t.Fatal(err)
	}
	rows, err := db.Query(`SELECT id, title, notes, completed, priority, due_date, created_at, updated_at FROM todos WHERE id = 1`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	_, err = scanTodoFromRows(rows)
	if err == nil {
		t.Fatal("expected error for invalid updated_at timestamp")
	}
	if err.Error() != "invalid updated_at timestamp: parsing time \"not-a-timestamp\" as \"2006-01-02T15:04:05Z07:00\": cannot parse \"not-a-timestamp\" as \"2006\"" {
		t.Fatalf("unexpected error message: %v", err)
	}
}

func newTestDB(t *testing.T) *sql.DB {
	t.Helper()
	tmp := t.TempDir()
	dbPath := filepath.Join(tmp, "test.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	db.SetMaxOpenConns(1)
	if err := Migrate(db); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	return NewStore(newTestDB(t), loc)
}

func TestCreate_valid(t *testing.T) {
	s := newTestStore(t)
	todo, err := s.Create("Buy milk", "2 liters", "medium", "2026-12-01")
	if err != nil {
		t.Fatal(err)
	}
	if todo.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if todo.Title != "Buy milk" {
		t.Errorf("got title %q", todo.Title)
	}
	if todo.Priority != "medium" {
		t.Errorf("got priority %q", todo.Priority)
	}
}

func TestCreate_emptyTitle(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Create("", "", "low", "")
	if err == nil {
		t.Fatal("expected error for empty title")
	}
}

func TestCreate_longTitle(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Create(strings.Repeat("x", 201), "", "low", "")
	if err == nil {
		t.Fatal("expected error for 201-char title")
	}
}

func TestCreate_invalidPriority(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Create("Title", "", "urgent", "")
	if err == nil {
		t.Fatal("expected error for invalid priority")
	}
}

func TestCreate_invalidDueDate(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Create("Title", "", "low", "not-a-date")
	if err == nil {
		t.Fatal("expected error for invalid due date")
	}
}

func TestUpdate_title(t *testing.T) {
	s := newTestStore(t)
	todo, _ := s.Create("Old title", "", "low", "")
	newTitle := "New title"
	updated, err := s.Update(todo.ID, UpdateReq{Title: &newTitle})
	if err != nil {
		t.Fatal(err)
	}
	if updated.Title != "New title" {
		t.Errorf("got %q", updated.Title)
	}
}

func TestUpdate_clearDueDate(t *testing.T) {
	s := newTestStore(t)
	todo, _ := s.Create("Task", "", "low", "2026-12-01")
	var nilStr *string
	updated, err := s.Update(todo.ID, UpdateReq{DueDate: &nilStr})
	if err != nil {
		t.Fatal(err)
	}
	if updated.DueDate != "" {
		t.Errorf("expected empty due date, got %q", updated.DueDate)
	}
}

func TestUpdate_notFound(t *testing.T) {
	s := newTestStore(t)
	title := "x"
	_, err := s.Update(9999, UpdateReq{Title: &title})
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestDelete_ok(t *testing.T) {
	s := newTestStore(t)
	todo, _ := s.Create("Delete me", "", "low", "")
	if err := s.Delete(todo.ID); err != nil {
		t.Fatal(err)
	}
	_, err := s.GetByID(todo.ID)
	if err != ErrNotFound {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestDelete_notFound(t *testing.T) {
	s := newTestStore(t)
	if err := s.Delete(9999); err != ErrNotFound {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestList_filter_status(t *testing.T) {
	s := newTestStore(t)
	t1, _ := s.Create("Active task", "", "low", "")
	t2, _ := s.Create("Done task", "", "low", "")
	tr := true
	s.Update(t2.ID, UpdateReq{Completed: &tr})

	active, err := s.List(ListFilter{Status: "active"})
	if err != nil {
		t.Fatal(err)
	}
	if len(active) != 1 || active[0].ID != t1.ID {
		t.Errorf("expected 1 active todo, got %d", len(active))
	}

	done, err := s.List(ListFilter{Status: "completed"})
	if err != nil {
		t.Fatal(err)
	}
	if len(done) != 1 || done[0].ID != t2.ID {
		t.Errorf("expected 1 completed todo, got %d", len(done))
	}
}

func TestList_search(t *testing.T) {
	s := newTestStore(t)
	s.Create("Buy groceries", "milk, eggs", "low", "")
	s.Create("Read book", "golang programming", "medium", "")

	results, err := s.List(ListFilter{Search: "milk"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 || results[0].Title != "Buy groceries" {
		t.Errorf("unexpected search results: %v", results)
	}
}

func TestList_sort(t *testing.T) {
	s := newTestStore(t)
	s.Create("Low prio", "", "low", "")
	s.Create("High prio", "", "high", "")
	s.Create("Medium prio", "", "medium", "")

	results, err := s.List(ListFilter{Sort: "priority"})
	if err != nil {
		t.Fatal(err)
	}
	if len(results) < 3 {
		t.Fatal("expected 3 results")
	}
	if results[0].Priority != "high" {
		t.Errorf("expected high first, got %q", results[0].Priority)
	}
	if results[1].Priority != "medium" {
		t.Errorf("expected medium second, got %q", results[1].Priority)
	}
	if results[2].Priority != "low" {
		t.Errorf("expected low third, got %q", results[2].Priority)
	}
}

func TestList_pagination(t *testing.T) {
	s := newTestStore(t)
	for i := 0; i < 5; i++ {
		s.Create("Task", "", "low", "")
	}

	page1, err := s.List(ListFilter{Page: 1, PerPage: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page1) != 3 {
		t.Errorf("expected 3, got %d", len(page1))
	}

	page2, err := s.List(ListFilter{Page: 2, PerPage: 3})
	if err != nil {
		t.Fatal(err)
	}
	if len(page2) != 2 {
		t.Errorf("expected 2, got %d", len(page2))
	}
}

func TestSummary(t *testing.T) {
	s := newTestStore(t)
	s.Create("Task 1", "", "low", "")
	s.Create("Task 2", "", "low", "")
	t3, _ := s.Create("Task 3", "", "low", "")
	tr := true
	s.Update(t3.ID, UpdateReq{Completed: &tr})

	active, completed, _, err := s.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if active != 2 {
		t.Errorf("expected 2 active, got %d", active)
	}
	if completed != 1 {
		t.Errorf("expected 1 completed, got %d", completed)
	}
}

func TestOverdue_timezone(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		t.Fatal(err)
	}
	s := NewStore(newTestDB(t), loc)

	// yesterday in Asia/Jakarta
	yesterday := time.Now().In(loc).AddDate(0, 0, -1).Format("2006-01-02")
	tomorrow := time.Now().In(loc).AddDate(0, 0, 1).Format("2006-01-02")

	s.Create("Overdue task", "", "high", yesterday)
	s.Create("Future task", "", "low", tomorrow)

	_, _, overdue, err := s.Summary()
	if err != nil {
		t.Fatal(err)
	}
	if overdue != 1 {
		t.Errorf("expected 1 overdue, got %d", overdue)
	}
}

func TestConcurrent(t *testing.T) {
	s := newTestStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 20)

	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := s.Create("Concurrent task", "", "medium", "")
			if err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)

	for err := range errs {
		t.Error(err)
	}

	todos, err := s.List(ListFilter{PerPage: 100})
	if err != nil {
		t.Fatal(err)
	}
	if len(todos) != 20 {
		t.Errorf("expected 20 todos, got %d", len(todos))
	}
}
func TestMigrate_idempotent(t *testing.T) {
	db := newTestDB(t) // already migrated once
	if err := Migrate(db); err != nil {
		t.Fatalf("second Migrate call failed: %v", err)
	}
}

func TestMigrate_WALMode(t *testing.T) {
	db := newTestDB(t)
	// migrate sets WAL mode
	if err := Migrate(db); err != nil {
		t.Fatalf("Migrate failed: %v", err)
	}
	var mode string
	if err := db.QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatal(err)
	}
	if strings.ToLower(mode) != "wal" {
		t.Errorf("expected journal_mode WAL, got %q", mode)
	}
}

// iterErrDriver simulates rows.Next returning false with rows.Err containing an error
type iterErrDriver struct{}

func (iterErrDriver) Open(name string) (driver.Conn, error) {
	return &iterErrConn{}, nil
}

type iterErrConn struct{}

func (c *iterErrConn) Prepare(query string) (driver.Stmt, error) {
	return &iterErrStmt{}, nil
}
func (c *iterErrConn) Close() error              { return nil }
func (c *iterErrConn) Begin() (driver.Tx, error) { return nil, fmt.Errorf("not supported") }

type iterErrStmt struct{}

func (s *iterErrStmt) Close() error  { return nil }
func (s *iterErrStmt) NumInput() int { return -1 }
func (s *iterErrStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, fmt.Errorf("not supported")
}
func (s *iterErrStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &iterErrRows{err: fmt.Errorf("row iteration error")}, nil
}

type iterErrRows struct {
	err       error
	hasResult bool
}

func (r *iterErrRows) Columns() []string {
	return []string{"id", "title", "notes", "completed", "priority", "due_date", "created_at", "updated_at"}
}
func (r *iterErrRows) Close() error { return nil }
func (r *iterErrRows) Next(dest []driver.Value) error {
	// First call succeeds with valid data
	if !r.hasResult {
		r.hasResult = true
		dest[0] = int64(1)
		dest[1] = "test"
		dest[2] = ""
		dest[3] = int64(0)
		dest[4] = "medium"
		dest[5] = ""
		dest[6] = "2026-11-04T07:40:00Z"
		dest[7] = "2026-11-04T07:40:00Z"
		return nil
	}
	// Second call signals iteration error so rows.Err() captures it
	return fmt.Errorf("row iteration error")
}
func (r *iterErrRows) Err() error {
	if r.hasResult && r.err == nil {
		r.err = fmt.Errorf("row iteration error")
	}
	return r.err
}
func init() {
	sql.Register("itertest", iterErrDriver{})
}

func TestList_queryError(t *testing.T) {
	db, err := sql.Open("itertest", "")
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	defer db.Close()
	s := NewStore(db, time.UTC)
	_, err = s.List(ListFilter{})
	if err == nil || err.Error() != "row iteration error" {
		t.Fatalf("expected row iteration error, got %v", err)
	}
}

// scanErrDriver simulates rows.Scan failure by returning incompatible type for ID
type scanErrDriver struct{}

func (scanErrDriver) Open(name string) (driver.Conn, error) {
	return &scanErrConn{}, nil
}

type scanErrConn struct{}

func (c *scanErrConn) Prepare(query string) (driver.Stmt, error) {
	return &scanErrStmt{}, nil
}
func (c *scanErrConn) Close() error              { return nil }
func (c *scanErrConn) Begin() (driver.Tx, error) { return nil, fmt.Errorf("not supported") }

type scanErrStmt struct{}

func (s *scanErrStmt) Close() error  { return nil }
func (s *scanErrStmt) NumInput() int { return -1 }
func (s *scanErrStmt) Exec(args []driver.Value) (driver.Result, error) {
	return nil, fmt.Errorf("not supported")
}
func (s *scanErrStmt) Query(args []driver.Value) (driver.Rows, error) {
	return &scanErrRows{}, nil
}

type scanErrRows struct{}

func (r *scanErrRows) Columns() []string {
	return []string{"id", "title", "notes", "completed", "priority", "due_date", "created_at", "updated_at"}
}
func (r *scanErrRows) Close() error { return nil }
func (r *scanErrRows) Next(dest []driver.Value) error {
	dest[0] = "not-an-int" // invalid type for int64 field
	dest[1] = "test"
	dest[2] = ""
	dest[3] = int64(0)
	dest[4] = "low"
	dest[5] = ""
	dest[6] = "2026-11-04T07:40:00Z"
	dest[7] = "2026-11-04T07:40:00Z"
	return nil
}
func init() {
	sql.Register("scanerrtest", scanErrDriver{})
}

func TestList_scanError(t *testing.T) {
	db, err := sql.Open("scanerrtest", "")
	if err != nil {
		t.Fatalf("sql.Open error: %v", err)
	}
	defer db.Close()
	s := NewStore(db, time.UTC)
	_, err = s.List(ListFilter{})
	if err == nil || err.Error() != `sql: Scan error on column index 0, name "id": converting driver.Value type string ("not-an-int") to a int64: invalid syntax` {
		t.Fatalf("expected scan error, got %v", err)
	}
}
