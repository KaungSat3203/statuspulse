package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net/http"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/gin-gonic/gin"
)

type AuthService interface {
	Signup(context.Context, string, string) (*models.User, error)
	Login(context.Context, string, string) (*models.User, error)
	CreateSession(context.Context, string) (string, time.Time, error)
	Authenticate(context.Context, string) (*models.Session, error)
	Logout(context.Context, string) error
	AllowAttempt(context.Context, string, string, string) (bool, error)
}

type AuthHandler struct {
	service  AuthService
	settings auth.Settings
}

func NewAuthHandler(service AuthService, settings auth.Settings) *AuthHandler {
	return &AuthHandler{service: service, settings: settings}
}

func (h *AuthHandler) cookie(c *gin.Context, value string, expires time.Time, delete bool) {
	age := int(h.settings.SessionTTL.Seconds())
	if delete {
		age = -1
		expires = time.Unix(1, 0).UTC()
	}
	http.SetCookie(c.Writer, &http.Cookie{Name: h.settings.CookieName(), Value: value,
		Path: "/", HttpOnly: true, Secure: h.settings.Secure, SameSite: http.SameSiteLaxMode,
		MaxAge: age, Expires: expires})
}

func (h *AuthHandler) Signup(c *gin.Context) { h.credentials(c, true) }
func (h *AuthHandler) Login(c *gin.Context)  { h.credentials(c, false) }
func (h *AuthHandler) credentials(c *gin.Context, signup bool) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 15*time.Second)
	defer cancel()
	var body struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 4096)
	decoder := json.NewDecoder(c.Request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		c.JSON(400, gin.H{"error": "invalid request body"})
		return
	}
	action := "login"
	if signup {
		action = "signup"
	}
	allowed, err := h.service.AllowAttempt(ctx, action, c.ClientIP(), body.Email)
	if err != nil {
		authError(c, err)
		return
	}
	if !allowed {
		c.Header("Retry-After", "900")
		c.JSON(429, gin.H{"error": "too many attempts; try again later"})
		return
	}
	var user *models.User
	if signup {
		user, err = h.service.Signup(ctx, body.Email, body.Password)
	} else {
		user, err = h.service.Login(ctx, body.Email, body.Password)
	}
	if err != nil {
		authError(c, err)
		return
	}
	// Never preserve an existing session token when authenticating again.
	old, _ := c.Cookie(h.settings.CookieName())
	if err := h.service.Logout(ctx, old); err != nil {
		authError(c, err)
		return
	}
	token, expires, err := h.service.CreateSession(ctx, user.ID)
	if err != nil {
		authError(c, err)
		return
	}
	h.cookie(c, token, expires, false)
	status := http.StatusOK
	if signup {
		status = http.StatusCreated
	}
	c.JSON(status, user)
}

func (h *AuthHandler) Logout(c *gin.Context) {
	token, _ := c.Cookie(h.settings.CookieName())
	if err := h.service.Logout(c.Request.Context(), token); err != nil {
		authError(c, err)
		return
	}
	h.cookie(c, "", time.Time{}, true)
	c.Status(http.StatusNoContent)
}

func (h *AuthHandler) Me(c *gin.Context) {
	session, ok := c.Get("auth.session")
	if !ok {
		authError(c, auth.ErrUnauthenticated)
		return
	}
	c.JSON(http.StatusOK, session.(*models.Session).User)
}

func (h *AuthHandler) Authenticate() gin.HandlerFunc {
	return func(c *gin.Context) {
		token, _ := c.Cookie(h.settings.CookieName())
		ctx, cancel := context.WithTimeout(c.Request.Context(), 5*time.Second)
		session, err := h.service.Authenticate(ctx, token)
		cancel()
		if err != nil {
			authError(c, err)
			return
		}
		c.Set("auth.session", session)
		c.Request = c.Request.WithContext(auth.WithUser(c.Request.Context(), session.User.ID))
		c.Next()
	}
}

// Explicit Origin + a non-simple custom header protect all cookie mutations,
// including login/logout. The proxy must forward, never fabricate, these values.
func (h *AuthHandler) BrowserSecurity() gin.HandlerFunc {
	allowed := map[string]bool{}
	for _, origin := range h.settings.Origins {
		allowed[origin] = true
	}
	return func(c *gin.Context) {
		c.Header("Cache-Control", "no-store")
		c.Header("X-Content-Type-Options", "nosniff")
		c.Writer.Header().Add("Vary", "Origin")
		origin := c.GetHeader("Origin")
		if allowed[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Allow-Headers", "Content-Type, X-StatusPulse-CSRF")
		}
		if c.Request.Method == http.MethodOptions {
			if !allowed[origin] {
				c.AbortWithStatusJSON(403, gin.H{"error": "origin not allowed"})
				return
			}
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		if c.Request.Method != http.MethodGet && c.Request.Method != http.MethodHead {
			if !allowed[origin] || c.GetHeader("X-StatusPulse-CSRF") != "1" {
				c.AbortWithStatusJSON(403, gin.H{"error": "invalid request origin or CSRF header"})
				return
			}
			if c.Request.Method == http.MethodPost || c.Request.Method == http.MethodPatch {
				mediaType, _, err := mime.ParseMediaType(c.GetHeader("Content-Type"))
				if err != nil || mediaType != "application/json" {
					c.AbortWithStatusJSON(415, gin.H{"error": "application/json required"})
					return
				}
			}
			c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 16*1024)
		} else if origin != "" && !allowed[origin] {
			c.AbortWithStatusJSON(403, gin.H{"error": "origin not allowed"})
			return
		}
		c.Next()
	}
}

func authError(c *gin.Context, err error) {
	status, message := http.StatusServiceUnavailable, "authentication temporarily unavailable"
	switch {
	case errors.Is(err, auth.ErrInvalidInput):
		status, message = 400, "use a valid email and a password between 15 and 128 characters"
	case errors.Is(err, auth.ErrDuplicateEmail):
		status, message = 409, "unable to create account with this email"
	case errors.Is(err, auth.ErrInvalidCredentials):
		status, message = 401, "invalid email or password"
	case errors.Is(err, auth.ErrUnauthenticated):
		status, message = 401, "authentication required"
	case errors.Is(err, auth.ErrBusy):
		c.Header("Retry-After", "5")
	}
	c.AbortWithStatusJSON(status, gin.H{"error": message})
}
