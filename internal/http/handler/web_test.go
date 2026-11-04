package handler_test

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/lukman-ss/ballo-todo/internal/app"
	"github.com/lukman-ss/ballo-todo/internal/config"
)

func newTestApp(t *testing.T) *app.App {
	t.Helper()
	_, currentFile, _, _ := runtime.Caller(0)
	templateDir := filepath.Join(filepath.Dir(currentFile), "../../web/templates")

	dir := t.TempDir()
	cfg := &config.Config{
		AppEnv:                "test",
		HTTPAddr:              ":0",
		HTTPReadHeaderTimeout: 5 * time.Second,
		HTTPReadTimeout:       10 * time.Second,
		HTTPWriteTimeout:      10 * time.Second,
		HTTPIdleTimeout:       120 * time.Second,
		ShutdownTimeout:       10 * time.Second,
		MaxBodyBytes:          1 << 20,
		LogLevel:              "error",
		DBPath:                filepath.Join(dir, "test.db"),
		Timezone:              "Asia/Jakarta",
		TemplateDir:           templateDir,
	}
	a, err := app.New(cfg)
	if err != nil {
		t.Fatalf("newTestApp: %v", err)
	}
	t.Cleanup(func() { a.Close() })
	return a
}

// getCSRF makes a GET / and returns the csrf_token cookie value.
func getCSRF(t *testing.T, srv http.Handler) string {
	t.Helper()
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	for _, c := range w.Result().Cookies() {
		if c.Name == "csrf_token" {
			return c.Value
		}
	}
	t.Fatal("no csrf_token cookie on GET /")
	return ""
}

func TestIndex_empty(t *testing.T) {
	a := newTestApp(t)
	req := httptest.NewRequest("GET", "/", nil)
	w := httptest.NewRecorder()
	a.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Ballo Todo") {
		t.Fatal("body missing 'Ballo Todo'")
	}
}

func TestNewForm(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	req := httptest.NewRequest("GET", "/todos/new", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
	w := httptest.NewRecorder()
	a.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "Tambah Todo") {
		t.Fatal("body missing 'Tambah Todo'")
	}
}

func postForm(t *testing.T, srv http.Handler, path string, vals url.Values, csrf string) *httptest.ResponseRecorder {
	t.Helper()
	body := vals.Encode()
	req := httptest.NewRequest("POST", path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
	w := httptest.NewRecorder()
	srv.ServeHTTP(w, req)
	return w
}

func TestCreate_quickAdd(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	w := postForm(t, a.Engine(), "/todos", url.Values{
		"csrf_token": {csrf},
		"title":      {"Test Todo"},
	}, csrf)

	if w.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d: %s", w.Code, w.Body.String())
	}
	if w.Header().Get("Location") != "/" {
		t.Fatalf("want redirect to /, got %s", w.Header().Get("Location"))
	}
}

func TestCreate_validation(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	// Quick add with empty title → redirect back with flash
	w := postForm(t, a.Engine(), "/todos", url.Values{
		"csrf_token": {csrf},
		"title":      {""},
	}, csrf)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d", w.Code)
	}
}

func TestCreate_fullFormValidation(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	// Full form with empty title → re-render form with errors
	w := postForm(t, a.Engine(), "/todos", url.Values{
		"csrf_token": {csrf},
		"title":      {""},
		"_source":    {"full"},
	}, csrf)
	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("want 422, got %d", w.Code)
	}
}

func TestEditForm(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	// Create todo first
	postForm(t, a.Engine(), "/todos", url.Values{
		"csrf_token": {csrf},
		"title":      {"Edit Me"},
	}, csrf)

	// Get ID from DB
	rows, err := a.DB().Query("SELECT id FROM todos LIMIT 1")
	if err != nil {
		t.Fatal(err)
	}
	var id int64
	rows.Next()
	rows.Scan(&id)
	rows.Close()

	req := httptest.NewRequest("GET", "/todos/1/edit", nil)
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
	w := httptest.NewRecorder()
	a.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("want 200, got %d: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "Edit Todo") {
		t.Fatal("body missing 'Edit Todo'")
	}
}

func TestEditSubmit(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	postForm(t, a.Engine(), "/todos", url.Values{
		"csrf_token": {csrf},
		"title":      {"Original"},
	}, csrf)

	w := postForm(t, a.Engine(), "/todos/1/edit", url.Values{
		"csrf_token": {csrf},
		"title":      {"Updated"},
		"priority":   {"high"},
		"notes":      {""},
		"due_date":   {""},
	}, csrf)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d: %s", w.Code, w.Body.String())
	}

	var title string
	a.DB().QueryRow("SELECT title FROM todos WHERE id=1").Scan(&title)
	if title != "Updated" {
		t.Fatalf("want title 'Updated', got %q", title)
	}
}

func TestToggle(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	postForm(t, a.Engine(), "/todos", url.Values{
		"csrf_token": {csrf},
		"title":      {"Toggle Me"},
	}, csrf)

	w := postForm(t, a.Engine(), "/todos/1/toggle", url.Values{
		"csrf_token": {csrf},
	}, csrf)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d", w.Code)
	}

	var completed int
	a.DB().QueryRow("SELECT completed FROM todos WHERE id=1").Scan(&completed)
	if completed != 1 {
		t.Fatal("todo should be completed after toggle")
	}
}

func TestDelete(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	postForm(t, a.Engine(), "/todos", url.Values{
		"csrf_token": {csrf},
		"title":      {"Delete Me"},
	}, csrf)

	w := postForm(t, a.Engine(), "/todos/1/delete", url.Values{
		"csrf_token": {csrf},
	}, csrf)
	if w.Code != http.StatusSeeOther {
		t.Fatalf("want 303, got %d", w.Code)
	}

	var count int
	a.DB().QueryRow("SELECT COUNT(*) FROM todos").Scan(&count)
	if count != 0 {
		t.Fatal("todo should be deleted")
	}
}

func TestCSRF_missing(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	// POST without csrf_token field (but with cookie so we get past cookie check)
	body := url.Values{"title": {"Test"}}.Encode()
	req := httptest.NewRequest("POST", "/todos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
	w := httptest.NewRecorder()
	a.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestCSRF_invalid(t *testing.T) {
	a := newTestApp(t)
	csrf := getCSRF(t, a.Engine())

	body := url.Values{"title": {"Test"}, "csrf_token": {"wrongtoken"}}.Encode()
	req := httptest.NewRequest("POST", "/todos", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: csrf})
	w := httptest.NewRecorder()
	a.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("want 403, got %d", w.Code)
	}
}

func TestCSRF_APIskipped(t *testing.T) {
	a := newTestApp(t)
	// POST to /api/v1/tasks without CSRF cookie or token should succeed
	body := strings.NewReader(`{"title":"test"}`)
	req := httptest.NewRequest("POST", "/api/v1/tasks", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	a.Engine().ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("want 201 for API route without CSRF, got %d", w.Code)
	}
}
