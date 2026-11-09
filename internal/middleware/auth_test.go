package middleware

import (
	"crypto/sha256"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

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
	// Each test starts with no remembered sessions.
	sessionCache.Lock()
	sessionCache.m = map[[32]byte]cachedSession{}
	sessionCache.Unlock()
	oldURL, oldKey := authServiceURL, authInternalKey
	authServiceURL, authInternalKey = srv.URL, "test-key"
	t.Cleanup(func() { authServiceURL, authInternalKey = oldURL, oldKey })
	return &calls
}

func callAPI(t *testing.T, withCookie bool) int {
	t.Helper()
	if !withCookie {
		return callAPIWith(t, "")
	}
	return callAPIWith(t, "token")
}

func callAPIWith(t *testing.T, cookie string) int {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/api/v1/order", RequireAuth(), func(c *gin.Context) {
		if _, ok := CurrentUserID(c); !ok {
			c.Status(http.StatusInternalServerError)
			return
		}
		c.Status(http.StatusOK)
	})
	req := httptest.NewRequest(http.MethodGet, "/api/v1/order", nil)
	if cookie != "" {
		req.AddCookie(&http.Cookie{Name: sessionCookieName, Value: cookie})
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

// A valid session is remembered for sessionCacheTTL: the next call doesn't
// ask the auth service again, but the identity is still set for handlers.
func TestValidSessionIsRememberedBriefly(t *testing.T) {
	calls := fakeAuth(t, true, 200)
	for i := 0; i < 3; i++ {
		if got := callAPIWith(t, "token-a"); got != http.StatusOK {
			t.Fatalf("call %d: status %d", i, got)
		}
	}
	if got := calls.Load(); got != 1 {
		t.Errorf("asked the auth service %d times, want 1", got)
	}

	// Once it has expired, the next call asks again.
	key := sha256.Sum256([]byte("token-a"))
	sessionCache.Lock()
	s := sessionCache.m[key]
	s.expires = time.Now().Add(-time.Second)
	sessionCache.m[key] = s
	sessionCache.Unlock()
	callAPIWith(t, "token-a")
	if got := calls.Load(); got != 2 {
		t.Errorf("after expiry: asked %d times, want 2", got)
	}

	// A different cookie never shares the entry.
	callAPIWith(t, "token-b")
	if got := calls.Load(); got != 3 {
		t.Errorf("other cookie: asked %d times, want 3", got)
	}
	sessionCache.Lock()
	for k := range sessionCache.m {
		if string(k[:]) == "token-a" || string(k[:]) == "token-b" {
			t.Error("the raw cookie is stored as a key")
		}
	}
	sessionCache.Unlock()
}

// A refusal or an auth-service error is never remembered: every call asks.
func TestRefusalsAreNeverRemembered(t *testing.T) {
	calls := fakeAuth(t, false, 200)
	callAPIWith(t, "dead")
	callAPIWith(t, "dead")
	if got := calls.Load(); got != 2 {
		t.Errorf("dead session: asked %d times, want 2", got)
	}

	calls = fakeAuth(t, true, 401)
	callAPIWith(t, "token")
	callAPIWith(t, "token")
	if got := calls.Load(); got != 2 {
		t.Errorf("auth error: asked %d times, want 2", got)
	}
}

// The cache stays bounded.
func TestSessionCacheIsCapped(t *testing.T) {
	sessionCache.Lock()
	sessionCache.m = map[[32]byte]cachedSession{}
	sessionCache.Unlock()
	for i := 0; i < sessionCacheMax+10; i++ {
		rememberValid(sha256.Sum256([]byte{byte(i), byte(i >> 8)}), cachedSession{id: 1, expires: time.Now().Add(time.Minute)})
	}
	sessionCache.Lock()
	n := len(sessionCache.m)
	sessionCache.Unlock()
	if n > sessionCacheMax {
		t.Errorf("cache holds %d entries, cap is %d", n, sessionCacheMax)
	}
}
