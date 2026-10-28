package app

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lukman-ss/ballo-todo/internal/config"
)

func newTestApp(t *testing.T) *App {
	t.Helper()
	cfg := &config.Config{
		AppEnv:                "test",
		HTTPAddr:              ":0",
		HTTPReadHeaderTimeout: 1000000000, // 1s
		HTTPReadTimeout:       1000000000,
		HTTPWriteTimeout:      1000000000,
		HTTPIdleTimeout:       120000000000,
		ShutdownTimeout:       1000000000,
		MaxBodyBytes:          1024,
		LogLevel:              "error",
	}
	a, err := New(cfg)
	if err != nil {
		t.Fatalf("failed to create app: %v", err)
	}
	return a
}

func request(t *testing.T, app *App, method, path string, body io.Reader) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	r := httptest.NewRequest(method, path, body)
	app.Engine().ServeHTTP(w, r)
	return w
}

func TestHealthz(t *testing.T) {
	app := newTestApp(t)
	w := request(t, app, "GET", "/healthz", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp map[string]string
	json.Unmarshal(w.Body.Bytes(), &resp)
	if resp["status"] != "pass" {
		t.Fatalf("expected status pass, got %q", resp["status"])
	}
}

func TestReadyzNotReady(t *testing.T) {
	app := newTestApp(t)
	app.SetReady(false)
	w := request(t, app, "GET", "/readyz", nil)
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503, got %d", w.Code)
	}
}

func TestReadyzReady(t *testing.T) {
	app := newTestApp(t)
	app.SetReady(true)
	w := request(t, app, "GET", "/readyz", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateAndGetTask(t *testing.T) {
	app := newTestApp(t)

	createPayload, _ := json.Marshal(map[string]string{"title": "Test task"})
	w := request(t, app, "POST", "/api/v1/tasks", bytes.NewReader(createPayload))
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var task map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &task); err != nil {
		t.Fatalf("failed to unmarshal response: %v", err)
	}
	if task["id"] == nil || task["title"] != "Test task" || task["done"] != false {
		t.Fatalf("unexpected task: %v", task)
	}
	taskID, _ := task["id"].(string)

	// GET task by ID
	w = request(t, app, "GET", "/api/v1/tasks/"+taskID, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var fetched map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &fetched)
	if fetched["id"] != taskID {
		t.Fatalf("expected id %s, got %v", taskID, fetched["id"])
	}
}

func TestListTasks(t *testing.T) {
	app := newTestApp(t)

	for i := 0; i < 3; i++ {
		payload, _ := json.Marshal(map[string]string{"title": "Task " + string(rune('A'+i))})
		w := request(t, app, "POST", "/api/v1/tasks", bytes.NewReader(payload))
		if w.Code != http.StatusCreated {
			t.Fatalf("create failed: %d", w.Code)
		}
	}

	w := request(t, app, "GET", "/api/v1/tasks", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var tasks []map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &tasks)
	if len(tasks) != 3 {
		t.Fatalf("expected 3 tasks, got %d", len(tasks))
	}
	// Verify deterministic order by ID
	if tasks[0]["id"] != "1" || tasks[1]["id"] != "2" || tasks[2]["id"] != "3" {
		t.Fatalf("unexpected order: %v", tasks)
	}
}

func TestUpdateTaskPartial(t *testing.T) {
	app := newTestApp(t)

	// Create task
	createPayload, _ := json.Marshal(map[string]string{"title": "Original"})
	w := request(t, app, "POST", "/api/v1/tasks", bytes.NewReader(createPayload))
	var created map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &created)
	taskID := created["id"].(string)

	// Patch only done field
	patchPayload, _ := json.Marshal(map[string]interface{}{"done": true})
	w = request(t, app, "PATCH", "/api/v1/tasks/"+taskID, bytes.NewReader(patchPayload))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var updated map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &updated)
	if updated["title"] != "Original" || updated["done"] != true {
		t.Fatalf("unexpected updated task: %v", updated)
	}
}

func TestDeleteTask(t *testing.T) {
	app := newTestApp(t)

	createPayload, _ := json.Marshal(map[string]string{"title": "Delete me"})
	w := request(t, app, "POST", "/api/v1/tasks", bytes.NewReader(createPayload))
	var created map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &created)
	taskID := created["id"].(string)

	// Delete task
	w = request(t, app, "DELETE", "/api/v1/tasks/"+taskID, nil)
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", w.Code, w.Body.String())
	}

	// Verify deleted
	w = request(t, app, "GET", "/api/v1/tasks/"+taskID, nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", w.Code)
	}
}

func TestValidationError(t *testing.T) {
	app := newTestApp(t)

	// Empty title
	w := request(t, app, "POST", "/api/v1/tasks", strings.NewReader(`{"title":""}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}

	// Title too long
	longTitle := strings.Repeat("x", 201)
	w = request(t, app, "POST", "/api/v1/tasks", strings.NewReader(`{"title":"`+longTitle+`"}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
}

func TestNotFound(t *testing.T) {
	app := newTestApp(t)

	w := request(t, app, "GET", "/nonexistent", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}

	// Nonexistent task ID
	w = request(t, app, "GET", "/api/v1/tasks/999", nil)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for missing task, got %d", w.Code)
	}
}

func TestMethodNotAllowed(t *testing.T) {
	app := newTestApp(t)

	w := request(t, app, "DELETE", "/api/v1/tasks", nil)
	if w.Code != http.StatusMethodNotAllowed {
		t.Fatalf("expected 405, got %d", w.Code)
	}
}

func TestMalformedJSON(t *testing.T) {
	app := newTestApp(t)

	w := request(t, app, "POST", "/api/v1/tasks", strings.NewReader(`{invalid json}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for malformed JSON, got %d", w.Code)
	}
}

func TestUnknownFields(t *testing.T) {
	app := newTestApp(t)

	w := request(t, app, "POST", "/api/v1/tasks", strings.NewReader(`{"title":"Test","unknown":"field"}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for unknown fields, got %d", w.Code)
	}
}

func TestEmptyBody(t *testing.T) {
	app := newTestApp(t)

	w := request(t, app, "POST", "/api/v1/tasks", strings.NewReader(``))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty body, got %d", w.Code)
	}
}

func TestUpdateEmptyFields(t *testing.T) {
	app := newTestApp(t)

	createPayload, _ := json.Marshal(map[string]string{"title": "Test"})
	w := request(t, app, "POST", "/api/v1/tasks", bytes.NewReader(createPayload))
	var created map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &created)
	taskID := created["id"].(string)

	// PATCH with no fields
	w = request(t, app, "PATCH", "/api/v1/tasks/"+taskID, strings.NewReader(`{}`))
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty patch, got %d", w.Code)
	}

	// PATCH with done=false
	w = request(t, app, "PATCH", "/api/v1/tasks/"+taskID, strings.NewReader(`{"done":false}`))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for patch done=false, got %d", w.Code)
	}
}
