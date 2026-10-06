package middleware

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"os"
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

		// Stash identity on the context so handlers (or RequireRole
		// below) can use it — e.g. for audit logging who created an
		// order, or restricting admin-only routes.
		c.Set("user_id", vr.User.ID)
		c.Set("user_email", vr.User.Email)
		c.Set("user_role", vr.User.Role)
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
