package web

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"strings"

	"github.com/lukman-ss/ballo"
)

// CSRFToken reads the CSRF token from the cookie, generating one if absent.
func CSRFToken(r *http.Request) (token string, isNew bool, err error) {
	if c, cerr := r.Cookie("csrf_token"); cerr == nil && c.Value != "" {
		return c.Value, false, nil
	}
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", false, err
	}
	return hex.EncodeToString(b), true, nil
}

// GetCSRFToken returns the token from cookie or "" if absent.
func GetCSRFToken(r *http.Request) string {
	c, err := r.Cookie("csrf_token")
	if err != nil {
		return ""
	}
	return c.Value
}

// SetCSRFCookie writes the CSRF cookie to the response.
func SetCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteStrictMode,
	})
}

// CSRFMiddleware enforces double-submit cookie CSRF protection on mutating methods.
// Skips paths under /api/ (JSON API uses other auth mechanisms).
func CSRFMiddleware() ballo.Middleware {
	return func(next ballo.Handler) ballo.Handler {
		return func(req *ballo.Request) (*ballo.Response, error) {
			// Skip CSRF for API routes
			if strings.HasPrefix(req.URL.Path, "/api/") {
				return next(req)
			}

			token, isNew, err := CSRFToken(req.Request)
			if err != nil {
				return ballo.HTML(http.StatusInternalServerError, "internal error"), nil
			}

			method := req.Method
			isMutating := method == "POST" || method == "PUT" || method == "PATCH" || method == "DELETE"

			if isMutating {
				_ = req.Request.ParseForm()
				formToken := req.Request.FormValue("csrf_token")
				cookieToken := GetCSRFToken(req.Request)
				if cookieToken == "" || formToken != cookieToken {
					return ballo.HTML(http.StatusForbidden,
						`<!DOCTYPE html><html><body><h1>403 Forbidden</h1><p>CSRF token tidak valid.</p><a href="/">← Kembali</a></body></html>`), nil
				}
			}

			resp, err := next(req)

			if isNew && resp != nil {
				if resp.Header == nil {
					resp.Header = make(http.Header)
				}
				c := &http.Cookie{
					Name:     "csrf_token",
					Value:    token,
					Path:     "/",
					HttpOnly: true,
					SameSite: http.SameSiteStrictMode,
				}
				resp.Header.Add("Set-Cookie", c.String())
			}

			return resp, err
		}
	}
}
