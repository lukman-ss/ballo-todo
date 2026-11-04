package web

import (
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/lukman-ss/ballo"
)

func TestCSRFToken_Generates(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	token, isNew, err := CSRFToken(req)
	if err != nil {
		t.Fatal(err)
	}
	if !isNew {
		t.Error("expected new token")
	}
	if len(token) != 64 {
		t.Errorf("expected 64-char token, got %d", len(token))
	}
}

func TestCSRFToken_ReusesExisting(t *testing.T) {
	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "abc"})
	token, isNew, err := CSRFToken(req)
	if err != nil {
		t.Fatal(err)
	}
	if isNew {
		t.Error("expected existing token not to be replaced")
	}
	if token != "abc" {
		t.Errorf("unexpected token %q", token)
	}
}

func TestCSRFCookie_Secure(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")
	w := httptest.NewRecorder()
	SetCSRFCookie(w, "tok")
	cookies := w.Result().Cookies()
	if len(cookies) != 1 {
		t.Fatalf("expected 1 cookie, got %d", len(cookies))
	}
	if !cookies[0].Secure {
		t.Error("expected Secure cookie in production")
	}
}

func TestCSRFMiddleware_GeneratesCookie(t *testing.T) {
	os.Setenv("APP_ENV", "production")
	defer os.Unsetenv("APP_ENV")

	h := CSRFMiddleware()(func(req *ballo.Request) (*ballo.Response, error) {
		return &ballo.Response{StatusCode: http.StatusOK}, nil
	})

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}
	br := &ballo.Request{Request: req}
	resp, err := h(br)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	found := false
	for _, line := range resp.Header["Set-Cookie"] {
		if strings.Contains(line, "csrf_token") && strings.Contains(line, "Secure") {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected Set-Cookie with csrf_token and Secure in production")
	}
}

func TestCSRFMiddleware_ValidPost(t *testing.T) {
	body := strings.NewReader("csrf_token=tok&foo=bar")
	req, err := http.NewRequest("POST", "/", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "tok"})

	called := false
	h := CSRFMiddleware()(func(req *ballo.Request) (*ballo.Response, error) {
		called = true
		return &ballo.Response{StatusCode: http.StatusOK}, nil
	})

	br := &ballo.Request{Request: req}
	resp, err := h(br)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Error("handler not called")
	}
}

func TestCSRFMiddleware_InvalidToken(t *testing.T) {
	body := strings.NewReader("csrf_token=bad&foo=bar")
	req, err := http.NewRequest("POST", "/", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.AddCookie(&http.Cookie{Name: "csrf_token", Value: "good"})

	h := CSRFMiddleware()(func(req *ballo.Request) (*ballo.Response, error) {
		t.Error("handler should not be called for invalid token")
		return &ballo.Response{StatusCode: http.StatusOK}, nil
	})

	br := &ballo.Request{Request: req}
	resp, err := h(br)
	if err != nil {
		t.Fatal(err)
	}
	if resp == nil {
		t.Fatal("expected response")
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCSRFMiddleware_ApiSkipped(t *testing.T) {
	body := strings.NewReader("foo=bar")
	req, err := http.NewRequest("POST", "/api/v1/tasks", body)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	called := false
	h := CSRFMiddleware()(func(req *ballo.Request) (*ballo.Response, error) {
		called = true
		return &ballo.Response{StatusCode: http.StatusOK}, nil
	})

	br := &ballo.Request{Request: req}
	resp, err := h(br)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if !called {
		t.Error("handler should be called for /api/ routes")
	}
}
