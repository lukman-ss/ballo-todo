package handler

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/lukman-ss/ballo"
	"github.com/lukman-ss/ballo-todo/internal/todo"
	"github.com/lukman-ss/ballo-todo/internal/web"
)

var indonesianMonthsWeb = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

func fmtDateIndonesian(s string) string {
	t, err := time.Parse("2006-01-02", s)
	if err != nil || s == "" {
		return ""
	}
	return fmt.Sprintf("%d %s %d", t.Day(), indonesianMonthsWeb[t.Month()], t.Year())
}

// WebHandler handles all web UI routes.
type WebHandler struct {
	store    *todo.Store
	renderer *web.Renderer
	loc      *time.Location
	maxBody  int64
}

func NewWebHandler(store *todo.Store, renderer *web.Renderer, loc *time.Location, maxBody int64) *WebHandler {
	return &WebHandler{store: store, renderer: renderer, loc: loc, maxBody: maxBody}
}

func (h *WebHandler) toView(t *todo.Todo) *web.TodoView {
	today := time.Now().In(h.loc).Format("2006-01-02")
	overdue := !t.Completed && t.DueDate != "" && t.DueDate < today
	return &web.TodoView{
		ID:               t.ID,
		Title:            t.Title,
		Notes:            t.Notes,
		Completed:        t.Completed,
		Priority:         t.Priority,
		DueDate:          t.DueDate,
		DueDateFormatted: fmtDateIndonesian(t.DueDate),
		Overdue:          overdue,
		CreatedAt:        t.CreatedAt.In(h.loc).Format("2 Jan 2006"),
	}
}

// Index handles GET /
func (h *WebHandler) Index(req *ballo.Request) (*ballo.Response, error) {
	q := req.URL.Query()
	filter := q.Get("filter")
	if filter == "" {
		filter = "all"
	}
	search := q.Get("q")
	sort := q.Get("sort")
	if sort == "" {
		sort = "newest"
	}
	page, _ := strconv.Atoi(q.Get("page"))
	if page < 1 {
		page = 1
	}

	todos, err := h.store.List(todo.ListFilter{
		Status:  filter,
		Search:  search,
		Sort:    sort,
		Page:    page,
		PerPage: 20,
	})
	if err != nil {
		return h.renderer.RenderError(http.StatusInternalServerError, "Gagal memuat daftar todo"), nil
	}

	active, completed, overdue, err := h.store.Summary()
	if err != nil {
		return h.renderer.RenderError(http.StatusInternalServerError, "Gagal memuat ringkasan"), nil
	}

	views := make([]*web.TodoView, len(todos))
	for i, t := range todos {
		views[i] = h.toView(t)
	}

	// Flash must be read using a real ResponseWriter — use a recorder to capture Set-Cookie
	// then copy into our ballo response.
	rec := httptest.NewRecorder()
	flash := web.GetFlash(rec, req.Request)

	resp, err := h.renderer.Render(http.StatusOK, "index", web.IndexData{
		Todos:     views,
		Summary:   web.SummaryData{Active: active, Completed: completed, Overdue: overdue},
		Filter:    filter,
		Search:    search,
		Sort:      sort,
		Page:      page,
		HasNext:   len(todos) == 20,
		HasPrev:   page > 1,
		CSRFToken: web.GetCSRFToken(req.Request),
		Flash:     flash,
	})
	if err != nil {
		return h.renderer.RenderError(http.StatusInternalServerError, err.Error()), nil
	}
	// Copy flash clear cookie
	for _, v := range rec.Result().Header["Set-Cookie"] {
		resp.Header.Add("Set-Cookie", v)
	}
	return resp, nil
}

// NewForm handles GET /todos/new
func (h *WebHandler) NewForm(req *ballo.Request) (*ballo.Response, error) {
	return h.renderer.Render(http.StatusOK, "new", web.FormData{
		CSRFToken: web.GetCSRFToken(req.Request),
		Errors:    map[string]string{},
	})
}

// Create handles POST /todos
func (h *WebHandler) Create(req *ballo.Request) (*ballo.Response, error) {
	if err := req.Request.ParseForm(); err != nil {
		return h.renderer.RenderError(http.StatusBadRequest, "Form tidak valid"), nil
	}

	title := strings.TrimSpace(req.Request.FormValue("title"))
	notes := req.Request.FormValue("notes")
	priority := req.Request.FormValue("priority")
	if priority == "" {
		priority = "medium"
	}
	dueDate := req.Request.FormValue("due_date")
	isFullForm := req.Request.FormValue("_source") == "full"

	errs := map[string]string{}
	if title == "" {
		errs["title"] = "Judul tidak boleh kosong"
	}
	if len(title) > 200 {
		errs["title"] = "Judul maksimal 200 karakter"
	}
	if priority != "low" && priority != "medium" && priority != "high" {
		priority = "medium"
	}

	if len(errs) > 0 {
		if !isFullForm {
			rec := httptest.NewRecorder()
			web.SetFlash(rec, "error", errs["title"])
			resp := redirect("/")
			for _, v := range rec.Result().Header["Set-Cookie"] {
				resp.Header.Add("Set-Cookie", v)
			}
			return resp, nil
		}
		return h.renderer.Render(http.StatusUnprocessableEntity, "new", web.FormData{
			Values:    web.FormValues{Title: title, Notes: notes, Priority: priority, DueDate: dueDate},
			Errors:    errs,
			CSRFToken: web.GetCSRFToken(req.Request),
		})
	}

	if _, err := h.store.Create(title, notes, priority, dueDate); err != nil {
		if isFullForm {
			errs["title"] = err.Error()
			return h.renderer.Render(http.StatusUnprocessableEntity, "new", web.FormData{
				Values:    web.FormValues{Title: title, Notes: notes, Priority: priority, DueDate: dueDate},
				Errors:    errs,
				CSRFToken: web.GetCSRFToken(req.Request),
			})
		}
		rec := httptest.NewRecorder()
		web.SetFlash(rec, "error", err.Error())
		resp := redirect("/")
		for _, v := range rec.Result().Header["Set-Cookie"] {
			resp.Header.Add("Set-Cookie", v)
		}
		return resp, nil
	}

	rec := httptest.NewRecorder()
	web.SetFlash(rec, "success", "Todo berhasil ditambahkan!")
	resp := redirect("/")
	for _, v := range rec.Result().Header["Set-Cookie"] {
		resp.Header.Add("Set-Cookie", v)
	}
	return resp, nil
}

// EditForm handles GET /todos/:id/edit
func (h *WebHandler) EditForm(req *ballo.Request) (*ballo.Response, error) {
	id, err := parseID(req)
	if err != nil {
		return h.renderer.RenderError(http.StatusBadRequest, "ID tidak valid"), nil
	}
	t, err := h.store.GetByID(id)
	if err != nil {
		if errors.Is(err, todo.ErrNotFound) {
			return h.renderer.RenderError(http.StatusNotFound, "Todo tidak ditemukan"), nil
		}
		return h.renderer.RenderError(http.StatusInternalServerError, err.Error()), nil
	}
	return h.renderer.Render(http.StatusOK, "edit", web.FormData{
		TodoID:    t.ID,
		Values:    web.FormValues{Title: t.Title, Notes: t.Notes, Priority: t.Priority, DueDate: t.DueDate},
		Errors:    map[string]string{},
		CSRFToken: web.GetCSRFToken(req.Request),
	})
}

// EditSubmit handles POST /todos/:id/edit
func (h *WebHandler) EditSubmit(req *ballo.Request) (*ballo.Response, error) {
	id, err := parseID(req)
	if err != nil {
		return h.renderer.RenderError(http.StatusBadRequest, "ID tidak valid"), nil
	}
	if err := req.Request.ParseForm(); err != nil {
		return h.renderer.RenderError(http.StatusBadRequest, "Form tidak valid"), nil
	}

	title := strings.TrimSpace(req.Request.FormValue("title"))
	notes := req.Request.FormValue("notes")
	priority := req.Request.FormValue("priority")
	dueDate := req.Request.FormValue("due_date")
	clearDueDate := req.Request.FormValue("clear_due_date") == "1"

	errs := map[string]string{}
	if title == "" {
		errs["title"] = "Judul tidak boleh kosong"
	}
	if len(title) > 200 {
		errs["title"] = "Judul maksimal 200 karakter"
	}
	if priority != "low" && priority != "medium" && priority != "high" {
		errs["priority"] = "Prioritas tidak valid"
	}

	if len(errs) > 0 {
		return h.renderer.Render(http.StatusUnprocessableEntity, "edit", web.FormData{
			TodoID:    id,
			Values:    web.FormValues{Title: title, Notes: notes, Priority: priority, DueDate: dueDate},
			Errors:    errs,
			CSRFToken: web.GetCSRFToken(req.Request),
		})
	}

	var dueDatePtr *string
	if clearDueDate {
		// outer ptr non-nil, inner nil → clear
		var inner *string
		dueDatePtr = inner
		updateReq := todo.UpdateReq{
			Title:    &title,
			Notes:    &notes,
			Priority: &priority,
			DueDate:  &dueDatePtr,
		}
		if _, err := h.store.Update(id, updateReq); err != nil {
			errs["title"] = err.Error()
			return h.renderer.Render(http.StatusUnprocessableEntity, "edit", web.FormData{
				TodoID:    id,
				Values:    web.FormValues{Title: title, Notes: notes, Priority: priority, DueDate: dueDate},
				Errors:    errs,
				CSRFToken: web.GetCSRFToken(req.Request),
			})
		}
	} else {
		dueDatePtr = &dueDate
		updateReq := todo.UpdateReq{
			Title:    &title,
			Notes:    &notes,
			Priority: &priority,
			DueDate:  &dueDatePtr,
		}
		if _, err := h.store.Update(id, updateReq); err != nil {
			errs["title"] = err.Error()
			return h.renderer.Render(http.StatusUnprocessableEntity, "edit", web.FormData{
				TodoID:    id,
				Values:    web.FormValues{Title: title, Notes: notes, Priority: priority, DueDate: dueDate},
				Errors:    errs,
				CSRFToken: web.GetCSRFToken(req.Request),
			})
		}
	}

	rec := httptest.NewRecorder()
	web.SetFlash(rec, "success", "Todo berhasil diperbarui!")
	resp := redirect("/")
	for _, v := range rec.Result().Header["Set-Cookie"] {
		resp.Header.Add("Set-Cookie", v)
	}
	return resp, nil
}

// Toggle handles POST /todos/:id/toggle
func (h *WebHandler) Toggle(req *ballo.Request) (*ballo.Response, error) {
	id, err := parseID(req)
	if err != nil {
		return h.renderer.RenderError(http.StatusBadRequest, "ID tidak valid"), nil
	}
	t, err := h.store.GetByID(id)
	if err != nil {
		if errors.Is(err, todo.ErrNotFound) {
			return redirect("/"), nil
		}
		return h.renderer.RenderError(http.StatusInternalServerError, err.Error()), nil
	}
	flipped := !t.Completed
	if _, err := h.store.Update(id, todo.UpdateReq{Completed: &flipped}); err != nil {
		return h.renderer.RenderError(http.StatusInternalServerError, err.Error()), nil
	}
	return redirect("/"), nil
}

// Delete handles POST /todos/:id/delete
func (h *WebHandler) Delete(req *ballo.Request) (*ballo.Response, error) {
	id, err := parseID(req)
	if err != nil {
		return h.renderer.RenderError(http.StatusBadRequest, "ID tidak valid"), nil
	}
	err = h.store.Delete(id)
	rec := httptest.NewRecorder()
	if err != nil && !errors.Is(err, todo.ErrNotFound) {
		web.SetFlash(rec, "error", "Gagal menghapus todo")
	} else {
		web.SetFlash(rec, "success", "Todo berhasil dihapus!")
	}
	resp := redirect("/")
	for _, v := range rec.Result().Header["Set-Cookie"] {
		resp.Header.Add("Set-Cookie", v)
	}
	return resp, nil
}

func parseID(req *ballo.Request) (int64, error) {
	return strconv.ParseInt(req.Param("id"), 10, 64)
}

func redirect(url string) *ballo.Response {
	return &ballo.Response{
		StatusCode: http.StatusSeeOther,
		Header:     http.Header{"Location": []string{url}},
	}
}
