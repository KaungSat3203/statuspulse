package repository

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/KaungSat3203/statuspulse/backend/internal/api"
	"github.com/KaungSat3203/statuspulse/backend/internal/auth"
	"github.com/KaungSat3203/statuspulse/backend/internal/models"
	"github.com/KaungSat3203/statuspulse/backend/internal/service"
	"github.com/KaungSat3203/statuspulse/backend/internal/telemetry"
	"github.com/gin-gonic/gin"
)

func TestAuthenticatedAPIIsolation(t *testing.T) {
	pool := newTestDatabase(t)
	ctx := context.Background()
	gin.SetMode(gin.TestMode)
	store := NewAuthRepository(pool)
	authService, err := auth.NewService(store, time.Hour, []byte(strings.Repeat("k", 32)))
	if err != nil {
		t.Fatal(err)
	}
	settings := auth.Settings{Origins: []string{"http://localhost:3000"}, SessionTTL: time.Hour}
	authHandler := api.NewAuthHandler(authService, settings)
	monitors := NewUserMonitorRepository(pool)
	monitorService := service.NewMonitorService(monitors)
	broadcaster := telemetry.NewBroadcaster()
	router := gin.New()
	api.RegisterRoutes(router,
		api.NewMonitorHandler(monitorService),
		api.NewCheckHistoryHandler(service.NewCheckHistoryService(monitors, NewUserCheckLogRepository(pool))),
		api.NewIncidentHandler(service.NewIncidentHistoryService(NewUserIncidentRepository(pool))),
		api.NewEventHandler(broadcaster, authHandler, monitorService),
		api.NewDailyMetricsHistoryHandler(service.NewDailyMetricsHistoryService(monitors, NewUserDailyMetricsRepository(pool))),
		authHandler)

	request := func(method, path, body string, cookie *http.Cookie) *httptest.ResponseRecorder {
		t.Helper()
		req := httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Origin", "http://localhost:3000")
		req.Header.Set("X-StatusPulse-CSRF", "1")
		req.Header.Set("Content-Type", "application/json")
		if cookie != nil {
			req.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		router.ServeHTTP(response, req)
		return response
	}
	signup := func(email string) (models.User, *http.Cookie) {
		t.Helper()
		res := request("POST", "/auth/signup", `{"email":"`+email+`","password":"correct horse battery staple"}`, nil)
		if res.Code != 201 {
			t.Fatalf("signup %d: %s", res.Code, res.Body)
		}
		var user models.User
		if err := json.Unmarshal(res.Body.Bytes(), &user); err != nil {
			t.Fatal(err)
		}
		return user, res.Result().Cookies()[0]
	}
	userA, cookieA := signup("a@example.com")
	userB, cookieB := signup("b@example.com")
	if res := request("POST", "/auth/signup", `{"email":"A@EXAMPLE.COM","password":"correct horse battery staple"}`, nil); res.Code != 409 {
		t.Fatal("duplicate not rejected")
	}
	for _, email := range []string{"a@example.com", "missing@example.com"} {
		res := request("POST", "/auth/login", `{"email":"`+email+`","password":"wrong password"}`, nil)
		if res.Code != 401 || !strings.Contains(res.Body.String(), "invalid email or password") {
			t.Fatal("non-generic login error")
		}
	}
	if res := request("POST", "/auth/login", `{"email":"a@example.com","password":"correct horse battery staple"}`, nil); res.Code != 200 {
		t.Fatal("valid login failed")
	}

	makeMonitor := func(cookie *http.Cookie) *models.Monitor {
		t.Helper()
		// A client cannot choose an owner, including by sending another user's ID.
		res := request("POST", "/monitors", `{"name":"test","url":"https://example.com","expected_status_code":200,"interval_seconds":30,"timeout_seconds":5,"user_id":"`+userB.ID+`"}`, cookie)
		if res.Code != 201 {
			t.Fatalf("create=%d %s", res.Code, res.Body)
		}
		var monitor models.Monitor
		if err := json.Unmarshal(res.Body.Bytes(), &monitor); err != nil {
			t.Fatal(err)
		}
		return &monitor
	}
	a, b := makeMonitor(cookieA), makeMonitor(cookieB)
	legacy := &models.Monitor{Name: "legacy", URL: "https://example.com", ExpectedStatusCode: 200, IntervalSeconds: 30, TimeoutSeconds: 5}
	if err := NewMonitorRepository(pool).Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	for _, m := range []*models.Monitor{a, b, legacy} {
		if _, err := NewCheckLogRepository(pool).CreateBatch(ctx, []models.CheckResult{{MonitorID: m.ID, CheckedAt: time.Now().UTC(), StatusCode: 200, LatencyMs: 10, Reachable: true}}); err != nil {
			t.Fatal(err)
		}
		if err := NewIncidentRepository(pool).Create(ctx, &models.Incident{MonitorID: m.ID, StartedAt: time.Now().UTC(), Reason: "test"}); err != nil {
			t.Fatal(err)
		}
		if _, err := NewDailyMetricsRepository(pool).Aggregate(ctx, m.ID, time.Now().UTC()); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := monitors.List(ctx); !errors.Is(err, auth.ErrUnauthenticated) {
		t.Fatal("repository allowed missing identity")
	}
	if got, err := NewMonitorRepository(pool).List(ctx); err != nil || len(got) != 3 {
		t.Fatal("engine lost global access")
	}
	for _, path := range []string{"/monitors", "/incidents", "/monitors/" + a.ID, "/monitors/" + a.ID + "/checks", "/monitors/" + a.ID + "/metrics", "/events", "/auth/me"} {
		if res := request("GET", path, "", nil); res.Code != 401 {
			t.Fatalf("unauthenticated %s=%d", path, res.Code)
		}
	}
	for _, id := range []string{b.ID, legacy.ID} {
		for _, tc := range []struct{ method, suffix, body string }{{"GET", "", ""}, {"GET", "/checks", ""}, {"GET", "/metrics", ""}, {"PATCH", "", `{"active":false}`}, {"DELETE", "", ""}} {
			if res := request(tc.method, "/monitors/"+id+tc.suffix, tc.body, cookieA); res.Code != 404 {
				t.Fatalf("IDOR %s %s=%d %s", tc.method, tc.suffix, res.Code, res.Body)
			}
		}
	}
	for _, path := range []string{"/monitors", "/incidents", "/monitors/" + a.ID + "/checks", "/monitors/" + a.ID + "/metrics"} {
		res := request("GET", path, "", cookieA)
		if res.Code != 200 || strings.Contains(res.Body.String(), b.ID) || strings.Contains(res.Body.String(), legacy.ID) {
			t.Fatalf("list isolation %s=%d %s", path, res.Code, res.Body)
		}
		var items []json.RawMessage
		if err := json.Unmarshal(res.Body.Bytes(), &items); err != nil || len(items) != 1 {
			t.Fatalf("expected one owned result for %s", path)
		}
	}
	// Direct history queries also scope ownership, even without service checks.
	aCtx := auth.WithUser(ctx, userA.ID)
	if got, err := NewUserCheckLogRepository(pool).ListByMonitorID(aCtx, b.ID, 50); err != nil || len(got) != 0 {
		t.Fatal("unscoped check query")
	}
	if got, err := NewUserDailyMetricsRepository(pool).ListByMonitorID(aCtx, b.ID, 30); err != nil || len(got) != 0 {
		t.Fatal("unscoped metric query")
	}
	if res := request("PATCH", "/monitors/"+a.ID, `{"name":"edited","active":false}`, cookieA); res.Code != 200 {
		t.Fatal("owned update failed")
	}
	if res := request("PATCH", "/monitors/"+a.ID, `{"active":true}`, cookieA); res.Code != 200 {
		t.Fatal("owned resume failed")
	}

	// An already-open SSE connection follows current ownership, not a snapshot.
	server := httptest.NewServer(router)
	defer server.Close()
	streamCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	streamReq, _ := http.NewRequestWithContext(streamCtx, "GET", server.URL+"/events", nil)
	streamReq.AddCookie(cookieA)
	stream, err := server.Client().Do(streamReq)
	if err != nil {
		t.Fatal(err)
	}
	defer stream.Body.Close()
	if stream.StatusCode != 200 || stream.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatal("invalid SSE response")
	}
	scanner := bufio.NewScanner(stream.Body)
	nextData := func() string {
		t.Helper()
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "data: ") {
				return strings.TrimPrefix(scanner.Text(), "data: ")
			}
		}
		t.Fatalf("missing SSE event: %v", scanner.Err())
		return ""
	}
	broadcaster.Publish(models.CheckResult{MonitorID: b.ID, Reachable: true})
	broadcaster.Publish(models.CheckResult{MonitorID: legacy.ID, Reachable: true})
	broadcaster.Publish(models.CheckResult{MonitorID: a.ID, Reachable: true})
	if data := nextData(); !strings.Contains(data, a.ID) {
		t.Fatalf("SSE leaked another owner: %s", data)
	}
	newMonitor := makeMonitor(cookieA)
	broadcaster.Publish(models.CheckResult{MonitorID: newMonitor.ID, Reachable: true})
	if data := nextData(); !strings.Contains(data, newMonitor.ID) {
		t.Fatal("new owned monitor missing from existing stream")
	}
	if res := request("DELETE", "/monitors/"+a.ID, "", cookieA); res.Code != 204 {
		t.Fatal("owned delete failed")
	}
	broadcaster.Publish(models.CheckResult{MonitorID: a.ID, Reachable: true})
	broadcaster.Publish(models.CheckResult{MonitorID: newMonitor.ID, Reachable: true})
	if data := nextData(); !strings.Contains(data, newMonitor.ID) {
		t.Fatal("deleted monitor event leaked")
	}
	if res := request("POST", "/auth/logout", "{}", cookieA); res.Code != 204 {
		t.Fatal("logout failed")
	}
	broadcaster.Publish(models.CheckResult{MonitorID: newMonitor.ID, Reachable: true})
	if data := nextData(); data != "{}" {
		t.Fatal("revoked session still received checks")
	}
	if res := request("GET", "/monitors", "", cookieA); res.Code != 401 {
		t.Fatal("revoked cookie accepted")
	}
	if _, err := pool.Exec(ctx, "UPDATE sessions SET expires_at=NOW()-interval '1 second' WHERE user_id=$1", userB.ID); err != nil {
		t.Fatal(err)
	}
	if res := request("GET", "/monitors", "", cookieB); res.Code != 401 {
		t.Fatal("expired cookie accepted")
	}
}
