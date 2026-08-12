package middleware

import (
	"bytes"
	"encoding/json"
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

// RequireAuth is the actual security boundary for this API — not
// nginx, not the frontend. It reads the session cookie the browser
// sent, forwards it server-to-server to KMA-auth's /internal/validate
// (gated there by AUTH_INTERNAL_KEY), and only lets the request
// through if that comes back valid. Anyone hitting these routes
// directly (curl, Postman, a scanner) without a real, live session
// gets rejected here regardless of how they got to this port.
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

		payload, _ := json.Marshal(validateRequest{Token: raw})
		req, err := http.NewRequest(http.MethodPost, authServiceURL+"/internal/validate", bytes.NewReader(payload))
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "auth check failed"})
			return
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Internal-Key", authInternalKey)

		resp, err := httpClient.Do(req)
		if err != nil {
			// Auth service down/unreachable — fail closed, not open.
			c.AbortWithStatusJSON(http.StatusBadGateway, gin.H{"error": "auth service unreachable"})
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "not authenticated"})
			return
		}

		var vr validateResponse
		if err := json.NewDecoder(resp.Body).Decode(&vr); err != nil || !vr.Valid {
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
