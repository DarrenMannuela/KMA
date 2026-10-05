package middleware

import (
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/gin-gonic/gin"
)

// fakeAuth stands in for KMA-auth's /internal/validate: it answers each
// call with the next status in statuses (the last one repeats), and a
// 200 says whether the session is valid.
func fakeAuth(t *testing.T, valid bool, statuses ...int) *atomic.Int32 {
	t.Helper()
	var calls atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(calls.Add(1)) - 1
		status := statuses[min(n, len(statuses)-1)]
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			if valid {
				w.Write([]byte(`{"valid":true,"user":{"id":1,"role":"staff","active":true}}`))
			} else {
				w.Write([]byte(`{"valid":false}`))
			}
		}
	}))
	t.Cleanup(srv.Close)
	oldURL, oldKey := authServiceURL, authInternalKey
	authServiceURL, authInternalKey = srv.URL, "test-key"
	t.Cleanup(func() { authServiceURL, authInternalKey = oldURL, oldKey })
	return &calls
}

func callAPI(t *testing.T, withCookie bool) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/order", RequireAuth(), func(c *gin.Context) { c.Status(http.StatusOK) })
	req := httptest.NewRequest(http.MethodGet, "/api/v1/order", nil)
	if withCookie {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: "token"})
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w.Code
}

func TestOnlyADeadSessionLogsOut(t *testing.T) {
	cases := []struct {
		name     string
		valid    bool
		statuses []int
		want     int
		calls    int32
	}{
		{"valid session", true, []int{200}, 200, 1},
		{"dead session", false, []int{200}, 401, 1},
		{"auth busy for a moment", true, []int{429, 200}, 200, 2},
		{"auth restarting for a moment", true, []int{503, 200}, 200, 2},
		{"auth busy for longer", true, []int{429}, 503, 2},
		{"internal key mismatch", true, []int{401}, 503, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := fakeAuth(t, tc.valid, tc.statuses...)
			if got := callAPI(t, true); got != tc.want {
				t.Errorf("status %d, want %d", got, tc.want)
			}
			if got := calls.Load(); got != tc.calls {
				t.Errorf("asked the auth service %d times, want %d", got, tc.calls)
			}
		})
	}
}

func TestNoCookieIsNotAuthenticated(t *testing.T) {
	calls := fakeAuth(t, true, 200)
	if got := callAPI(t, false); got != http.StatusUnauthorized {
		t.Errorf("status %d, want 401", got)
	}
	if calls.Load() != 0 {
		t.Error("asked the auth service without a session to check")
	}
}

func TestAuthServiceDownFailsClosed(t *testing.T) {
	fakeAuth(t, true, 200)
	authServiceURL = "http://127.0.0.1:1" // nothing listens there
	if got := callAPI(t, true); got != http.StatusBadGateway {
		t.Errorf("status %d, want 502", got)
	}
}
