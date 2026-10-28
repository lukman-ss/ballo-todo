package web

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/lukman-ss/ballo"
)

var indonesianMonths = [...]string{
	"", "Januari", "Februari", "Maret", "April", "Mei", "Juni",
	"Juli", "Agustus", "September", "Oktober", "November", "Desember",
}

// Renderer renders HTML templates using base.html + per-page file cloning.
type Renderer struct {
	baseTmpl    *template.Template
	templateDir string
	loc         *time.Location
}

func NewRenderer(templateDir string, loc *time.Location) (*Renderer, error) {
	funcMap := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"fmtDate": func(s string) string {
			t, err := time.Parse("2006-01-02", s)
			if err != nil || s == "" {
				return s
			}
			return fmt.Sprintf("%d %s %d", t.Day(), indonesianMonths[t.Month()], t.Year())
		},
		"truncate": func(s string, n int) string {
			runes := []rune(s)
			if len(runes) <= n {
				return s
			}
			return string(runes[:n]) + "…"
		},
	}

	basePath := filepath.Join(templateDir, "base.html")
	base, err := template.New("base").Funcs(funcMap).ParseFiles(basePath)
	if err != nil {
		return nil, fmt.Errorf("parse base.html: %w", err)
	}

	return &Renderer{
		baseTmpl:    base,
		templateDir: templateDir,
		loc:         loc,
	}, nil
}

func (r *Renderer) Render(status int, page string, data interface{}) (*ballo.Response, error) {
	pagePath := filepath.Join(r.templateDir, page+".html")
	tmpl, err := r.baseTmpl.Clone()
	if err != nil {
		return nil, fmt.Errorf("clone base template: %w", err)
	}
	tmpl, err = tmpl.ParseFiles(pagePath)
	if err != nil {
		return nil, fmt.Errorf("parse %s.html: %w", page, err)
	}

	var buf bytes.Buffer
	if err := tmpl.ExecuteTemplate(&buf, "base", data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return ballo.HTML(status, buf.String()), nil
}

func (r *Renderer) RenderError(status int, message string) *ballo.Response {
	body := fmt.Sprintf(`<!DOCTYPE html><html><head><title>Error %d</title></head><body><h1>Error %d</h1><p>%s</p><a href="/">← Kembali</a></body></html>`,
		status, status, template.HTMLEscapeString(message))
	return ballo.HTML(status, body)
}

// FlashMsg carries a flash notification.
type FlashMsg struct {
	Type    string
	Message string
}

// SummaryData holds todo counts.
type SummaryData struct {
	Active, Completed, Overdue int
}

// TodoView is the presentation model for a single todo.
type TodoView struct {
	ID               int64
	Title, Notes     string
	Completed        bool
	Priority         string
	DueDate          string
	DueDateFormatted string
	Overdue          bool
	CreatedAt        string
}

// IndexData is the template data for the index page.
type IndexData struct {
	Todos     []*TodoView
	Summary   SummaryData
	Filter    string
	Search    string
	Sort      string
	Page      int
	HasNext   bool
	HasPrev   bool
	CSRFToken string
	Flash     FlashMsg
}

// FormValues holds form field values for create/edit forms.
type FormValues struct {
	Title, Notes, Priority, DueDate string
}

// FormData is the template data for create/edit forms.
type FormData struct {
	TodoID    int64
	Values    FormValues
	Errors    map[string]string
	CSRFToken string
	Flash     FlashMsg
}

// SetFlash sets a flash cookie.
func SetFlash(w http.ResponseWriter, typ, msg string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "flash",
		Value:    typ + "|" + strings.ReplaceAll(msg, "|", " "),
		Path:     "/",
		MaxAge:   10,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// GetFlash reads and clears the flash cookie.
func GetFlash(w http.ResponseWriter, r *http.Request) FlashMsg {
	c, err := r.Cookie("flash")
	if err != nil {
		return FlashMsg{}
	}
	// Clear it
	http.SetCookie(w, &http.Cookie{
		Name:     "flash",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
	parts := strings.SplitN(c.Value, "|", 2)
	if len(parts) != 2 {
		return FlashMsg{}
	}
	return FlashMsg{Type: parts[0], Message: parts[1]}
}
