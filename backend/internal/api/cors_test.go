package api

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestLocalFrontendCORSPreflightForMonitorCreation(t *testing.T) {
	router := corsTestRouter()

	request := httptest.NewRequest(http.MethodOptions, "/monitors", nil)
	request.Header.Set("Origin", localFrontendOrigin)
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != localFrontendOrigin {
		t.Fatalf(
			"Access-Control-Allow-Origin = %q, want %q",
			got,
			localFrontendOrigin,
		)
	}
	if methods := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(methods, http.MethodPost) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want POST", methods)
	}
	if headers := recorder.Header().Get("Access-Control-Allow-Headers"); !strings.Contains(headers, "Content-Type") {
		t.Fatalf("Access-Control-Allow-Headers = %q, want Content-Type", headers)
	}
}

func TestLocalFrontendCORSAllowsMonitorPost(t *testing.T) {
	router := corsTestRouter()

	request := httptest.NewRequest(http.MethodPost, "/monitors", nil)
	request.Header.Set("Origin", localFrontendOrigin)
	request.Header.Set("Content-Type", "application/json")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != localFrontendOrigin {
		t.Fatalf(
			"Access-Control-Allow-Origin = %q, want %q",
			got,
			localFrontendOrigin,
		)
	}
}

func TestLocalFrontendCORSRejectsUnrelatedOrigin(t *testing.T) {
	router := corsTestRouter()

	request := httptest.NewRequest(http.MethodOptions, "/monitors", nil)
	request.Header.Set("Origin", "http://example.com")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if got := recorder.Header().Get("Access-Control-Allow-Origin"); got != "" {
		t.Fatalf("Access-Control-Allow-Origin = %q, want empty", got)
	}
}

func corsTestRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)

	router := gin.New()
	router.Use(testBrowserSecurity())
	router.POST("/monitors", func(c *gin.Context) {
		c.Status(http.StatusCreated)
	})
	router.GET("/events", func(c *gin.Context) {
		c.Status(http.StatusOK)
	})

	return router
}

func TestLocalFrontendCORSAllowsMonitorPatch(t *testing.T) {
	router := corsTestRouter()

	request := httptest.NewRequest(http.MethodOptions, "/monitors/monitor-id", nil)
	request.Header.Set("Origin", localFrontendOrigin)
	request.Header.Set("Access-Control-Request-Method", http.MethodPatch)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type")

	recorder := httptest.NewRecorder()
	router.ServeHTTP(recorder, request)

	if recorder.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusNoContent)
	}
	if methods := recorder.Header().Get("Access-Control-Allow-Methods"); !strings.Contains(methods, http.MethodPatch) {
		t.Fatalf("Access-Control-Allow-Methods = %q, want PATCH", methods)
	}
}
