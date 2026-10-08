package api

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/gin-gonic/gin"
)

const localFrontendOrigin = "http://localhost:3000"

func testBrowserSecurity() gin.HandlerFunc {
	return NewAuthHandler(&fakeAuth{allowed: true}, auth.Settings{Origins: []string{localFrontendOrigin}}).BrowserSecurity()
}

type fakeAuth struct {
	failure error
	allowed bool
	revoked bool
}

func (f *fakeAuth) Signup(context.Context, string, string) (*models.User, error) {
	return &models.User{ID: "user-a", Email: "a@example.com", PasswordHash: "never-expose"}, f.failure
}
func (f *fakeAuth) Login(ctx context.Context, email, password string) (*models.User, error) {
	return f.Signup(ctx, email, password)
}
func (f *fakeAuth) CreateSession(context.Context, string) (string, time.Time, error) {
	f.revoked = false
	return "opaque-cookie", time.Now().Add(time.Hour), f.failure
}
func (f *fakeAuth) Authenticate(_ context.Context, token string) (*models.Session, error) {
	if f.failure != nil {
		return nil, f.failure
	}
	if token != "opaque-cookie" || f.revoked {
		return nil, auth.ErrUnauthenticated
	}
	return &models.Session{User: models.User{ID: "user-a", Email: "a@example.com"}, ExpiresAt: time.Now().Add(time.Hour)}, nil
}
func (f *fakeAuth) Logout(context.Context, string) error { f.revoked = true; return f.failure }
func (f *fakeAuth) AllowAttempt(context.Context, string, string, string) (bool, error) {
	return f.allowed, f.failure
}

func authTestRouter(f *fakeAuth, secure bool) (*gin.Engine, *AuthHandler) {
	gin.SetMode(gin.TestMode)
	h := NewAuthHandler(f, auth.Settings{Secure: secure, Origins: []string{localFrontendOrigin}, SessionTTL: time.Hour})
	r := gin.New()
	r.Use(h.BrowserSecurity())
	r.POST("/auth/signup", h.Signup)
	r.POST("/auth/login", h.Login)
	r.POST("/auth/logout", h.Logout)
	r.GET("/auth/me", h.Authenticate(), h.Me)
	return r, h
}

func authRequest(r http.Handler, method, path, origin, csrf, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, path, strings.NewReader(body))
	if origin != "" {
		request.Header.Set("Origin", origin)
	}
	if csrf != "" {
		request.Header.Set("X-StatusPulse-CSRF", csrf)
	}
	request.Header.Set("Content-Type", "application/json")
	if cookie != nil {
		request.AddCookie(cookie)
	}
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	return response
}

func TestAuthCookiesAndRevocation(t *testing.T) {
	for _, secure := range []bool{false, true} {
		f := &fakeAuth{allowed: true}
		r, h := authTestRouter(f, secure)
		response := authRequest(r, "POST", "/auth/signup", localFrontendOrigin, "1", `{"email":"a@example.com","password":"a long password"}`, nil)
		if response.Code != 201 {
			t.Fatalf("signup=%d %s", response.Code, response.Body)
		}
		cookies := response.Result().Cookies()
		if len(cookies) != 1 {
			t.Fatal("missing cookie")
		}
		cookie := cookies[0]
		if cookie.Name != h.settings.CookieName() || !cookie.HttpOnly || cookie.Secure != secure || cookie.Path != "/" || cookie.Domain != "" ||
			cookie.SameSite != http.SameSiteLaxMode || cookie.MaxAge != 3600 || cookie.Expires.IsZero() {
			t.Fatal("unsafe cookie settings")
		}
		if strings.Contains(response.Body.String(), "opaque-cookie") || strings.Contains(response.Body.String(), "never-expose") {
			t.Fatal("secret in response")
		}
		me := authRequest(r, "GET", "/auth/me", "", "", "", cookie)
		if me.Code != 200 {
			t.Fatalf("me=%d", me.Code)
		}
		logout := authRequest(r, "POST", "/auth/logout", localFrontendOrigin, "1", "{}", cookie)
		if logout.Code != 204 || logout.Result().Cookies()[0].MaxAge != -1 {
			t.Fatal("logout did not clear cookie")
		}
		if got := authRequest(r, "GET", "/auth/me", "", "", "", cookie).Code; got != 401 {
			t.Fatalf("revoked status=%d", got)
		}
	}
}

func TestAuthSecurityAndErrors(t *testing.T) {
	f := &fakeAuth{allowed: true}
	r, _ := authTestRouter(f, false)
	for _, tc := range []struct{ origin, csrf string }{{"", "1"}, {"null", "1"}, {"https://evil.example", "1"}, {localFrontendOrigin, ""}} {
		response := authRequest(r, "POST", "/auth/login", tc.origin, tc.csrf, "{}", nil)
		if response.Code != 403 {
			t.Fatalf("CSRF accepted: %+v", tc)
		}
	}
	request := httptest.NewRequest("OPTIONS", "/monitors", nil)
	request.Header.Set("Origin", localFrontendOrigin)
	response := httptest.NewRecorder()
	r.ServeHTTP(response, request)
	if response.Code != 204 || response.Header().Get("Access-Control-Allow-Origin") != localFrontendOrigin ||
		response.Header().Get("Access-Control-Allow-Credentials") != "true" ||
		!strings.Contains(response.Header().Get("Access-Control-Allow-Headers"), "X-StatusPulse-CSRF") {
		t.Fatal("invalid preflight")
	}
	if got := authRequest(r, "GET", "/auth/me", "", "", "", nil).Code; got != 401 {
		t.Fatal("unauthenticated access allowed")
	}
	for _, tc := range []struct {
		err    error
		status int
	}{
		{auth.ErrInvalidInput, 400}, {auth.ErrDuplicateEmail, 409}, {auth.ErrInvalidCredentials, 401},
		{errors.New("database password=secret"), 503},
	} {
		// Error from limiter is also safely classified.
		f.failure = tc.err
		response := authRequest(r, "POST", "/auth/login", localFrontendOrigin, "1", "{}", nil)
		if response.Code != tc.status || strings.Contains(response.Body.String(), "secret") {
			t.Fatalf("unsafe error: %d %s", response.Code, response.Body)
		}
	}
	f.failure = nil
	f.allowed = false
	if response := authRequest(r, "POST", "/auth/login", localFrontendOrigin, "1", "{}", nil); response.Code != 429 || response.Header().Get("Retry-After") == "" {
		t.Fatal("missing rate limit")
	}
	for _, body := range []string{`{}{}`, `{"unknown":true}`, strings.Repeat("a", 5000)} {
		if got := authRequest(r, "POST", "/auth/signup", localFrontendOrigin, "1", body, nil).Code; got != 400 {
			t.Fatalf("invalid body=%d", got)
		}
	}
}
