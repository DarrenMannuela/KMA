package middleware

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

// This must match KMA-auth's mw.SessionCookieName exactly — both
// services set/read the same cookie, since it's the same browser
// session shared across same-origin routes behind nginx.
const sessionCookieName = "kma_session"

var (
	authServiceURL  = mustGetEnv("AUTH_SERVICE_URL", "http://kma_auth_backend:8001")
	authInternalKey = os.Getenv("AUTH_INTERNAL_KEY")
	httpClient      = &http.Client{Timeout: 3 * time.Second}
)

func mustGetEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// A "valid" answer from KMA-auth is reused for sessionCacheTTL instead of
// asking again on every API call: each ask is an HTTP hop plus database work
// on the auth side, and one page load makes about ten of them. The cost,
// agreed with the owner, is that a logout, deactivation or single-device
// sign-out reaches this API up to 15 seconds late (logout already clears the
// browser's cookie at once). Only valid answers are cached, never a refusal
// or an auth-service error, and the key is a hash of the cookie, never the
// cookie itself.
const (
	sessionCacheTTL = 15 * time.Second
	sessionCacheMax = 1000
)

type cachedSession struct {
	id      uint
	email   string
	role    string
	expires time.Time
}

var sessionCache = struct {
	sync.Mutex
	m map[[32]byte]cachedSession
}{m: map[[32]byte]cachedSession{}}

func cachedValid(key [32]byte, now time.Time) (cachedSession, bool) {
	sessionCache.Lock()
	defer sessionCache.Unlock()
	s, ok := sessionCache.m[key]
	if !ok {
		return cachedSession{}, false
	}
	if now.After(s.expires) {
		delete(sessionCache.m, key)
		return cachedSession{}, false
	}
	return s, true
}

func rememberValid(key [32]byte, s cachedSession) {
	sessionCache.Lock()
	defer sessionCache.Unlock()
	if len(sessionCache.m) >= sessionCacheMax {
		now := time.Now()
		for k, v := range sessionCache.m {
			if now.After(v.expires) {
				delete(sessionCache.m, k)
			}
		}
		if len(sessionCache.m) >= sessionCacheMax {
			sessionCache.m = map[[32]byte]cachedSession{}
		}
	}
	sessionCache.m[key] = s
}

func setIdentity(c *gin.Context, id uint, email, role string) {
	// Stash identity on the context so handlers (or RequireRole
	// below) can use it — e.g. for audit logging who created an
	// order, or restricting admin-only routes.
	c.Set("user_id", id)
	c.Set("user_email", email)
	c.Set("user_role", role)
}

type validateRequest struct {
	Token string `json:"token"`
}

type validateResponse struct {
	Valid bool `json:"valid"`
	User  struct {
		ID     uint   `json:"id"`
		Email  string `json:"email"`
		Name   string `json:"name"`
		Role   string `json:"role"`
		Active bool   `json:"active"`
	} `json:"user"`
}

// RequireAuth is the API's security boundary: it forwards the session
// cookie to KMA-auth's /internal/validate (with AUTH_INTERNAL_KEY) and only
// lets valid sessions through.
func RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if authInternalKey == "" {
			// Fail closed: an unset key must never silently become
			// "let everyone through". Mirrors KMA-auth's own
			// RequireInternalKey behavior on the other side.
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth not configured"})
			return
		}

		raw, err := c.Cookie(sessionCookieName)
		if err != nil || raw == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		key := sha256.Sum256([]byte(raw))
		if s, ok := cachedValid(key, time.Now()); ok {
			setIdentity(c, s.id, s.email, s.role)
			c.Next()
			return
		}

		resp, err := askAuthService(raw)
		if err != nil {
			// Auth service down/unreachable even after a retry — fail
			// closed, not open.
			log.Printf("auth check: auth service unreachable: %v", err)
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "auth service unreachable"})
			return
		}
		defer resp.Body.Close()

		// Only 200 with valid=false means the session is dead. Any other status is
		// about the auth service itself (rate limit, key mismatch): refuse with 503,
		// which the frontend shows as retryable, not 401, which logs people out.
		if resp.StatusCode != http.StatusOK {
			log.Printf("auth check: auth service answered %d", resp.StatusCode)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth service busy, try again in a moment"})
			return
		}

		var vr validateResponse
		if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil {
			log.Printf("auth check: unreadable answer from auth service: %v", err)
			c.AbortWithStatusJSON(http.StatusServiceUnavailable, gin.H{"error": "auth service busy, try again in a moment"})
			return
		}
		if !vr.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		rememberValid(key, cachedSession{id: vr.User.ID, email: vr.User.Email, role: vr.User.Role, expires: time.Now().Add(sessionCacheTTL)})
		setIdentity(c, vr.User.ID, vr.User.Email, vr.User.Role)
		c.Next()
	}
}

// askAuthService asks KMA-auth to validate a session token, retrying once
// after a short pause if the service is unreachable or answers 429/502/503/504
// (a restart or a burst).
func askAuthService(token string) (*http.Response, error) {
	payload, err := json.Marshal(validateRequest{Token: token})
	if err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		if attempt > 0 {
			time.Sleep(300 * time.Millisecond)
		}
		req, err := http.NewRequest(http.MethodPost, authServiceURL+"/internal/validate", bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Key", authInternalKey)

		resp, err := httpClient.Do(req)
		if err != nil {
			lastErr = err
			continue
		}
		switch resp.StatusCode {
		case http.StatusTooManyRequests, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			if attempt == 0 {
				resp.Body.Close()
				continue
			}
		}
		return resp, nil
	}
	return nil, lastErr
}

// RequireRole gates a route to specific roles. Must run after
// RequireAuth, same pattern as KMA-auth's own middleware.
func RequireRole(roles ...string) gin.HandlerFunc {
	allowed := make(map[string]bool, len(roles))
	for _, r := range roles {
		allowed[r] = true
	}
	return func(c *gin.Context) {
		roleVal, ok := c.Get("user_role")
		role, _ := roleVal.(string)
		if !ok || !allowed[role] {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "insufficient permissions"})
			return
		}
		c.Next()
	}
}

// CurrentUserID is a small helper for handlers that want to know who
// made the request (e.g. for audit fields).
func CurrentUserID(c *gin.Context) (uint, bool) {
	v, ok := c.Get("user_id")
	if !ok {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok
}
