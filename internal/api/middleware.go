package api

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
)

const userIDKey = "userID"
const userRoleKey = "userRole"

var errUnauthorized = errors.New("unauthorized")
var errBanned = errors.New("banned")

func (s *Server) authMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		userID, role, err := s.authenticateRequest(c)
		if err != nil {
			if errors.Is(err, errBanned) {
				return c.JSON(http.StatusForbidden, map[string]string{"error": "your account has been banned"})
			}
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		}
		c.Set(userIDKey, userID)
		c.Set(userRoleKey, role)
		return next(c)
	}
}

func (s *Server) authenticateRequest(c echo.Context) (int64, string, error) {
	tokenStr := ""
	authHeader := c.Request().Header.Get("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		tokenStr = strings.TrimSpace(authHeader[7:])
	}
	if tokenStr == "" {
		cookie, err := c.Request().Cookie("token")
		if err == nil {
			tokenStr = cookie.Value
		}
	}
	if tokenStr == "" {
		return 0, "", errUnauthorized
	}

	claims := jwt.MapClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
		if t.Method != jwt.SigningMethodHS256 {
			return nil, fmt.Errorf("unexpected signing method: %s", t.Method.Alg())
		}
		return []byte(s.cfg.Server.SecretKey), nil
	})
	if err != nil || !token.Valid {
		return 0, "", errUnauthorized
	}

	userIDFloat, ok := claims["user_id"].(float64)
	if !ok || userIDFloat <= 0 {
		return 0, "", errUnauthorized
	}
	role, ok := claims["role"].(string)
	if !ok || role == "" {
		return 0, "", errUnauthorized
	}
	userID := int64(userIDFloat)

	// Deny access to banned users (DB check; fast because id is PK-indexed).
	var banned bool
	_ = s.db.QueryRow("SELECT COALESCE(banned, 0) FROM users WHERE id=?", userID).Scan(&banned)
	if banned {
		return 0, "", errBanned
	}

	return userID, role, nil
}

func (s *Server) adminMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		role, _ := c.Get(userRoleKey).(string)
		if role != "admin" {
			return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
		}
		return next(c)
	}
}

type rateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
}

type bucket struct {
	count   int
	resetAt time.Time
}

func newRateLimiter() *rateLimiter {
	rl := &rateLimiter{buckets: make(map[string]*bucket)}
	go rl.cleanup()
	return rl
}

func (rl *rateLimiter) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		rl.mu.Lock()
		now := time.Now()
		for k, b := range rl.buckets {
			if now.After(b.resetAt) {
				delete(rl.buckets, k)
			}
		}
		rl.mu.Unlock()
	}
}

func (rl *rateLimiter) allow(key string, limit int, window time.Duration) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[key]
	if !ok || time.Now().After(b.resetAt) {
		rl.buckets[key] = &bucket{count: 1, resetAt: time.Now().Add(window)}
		return true
	}
	if b.count >= limit {
		return false
	}
	b.count++
	return true
}

// getClientIP returns the real client IP from RemoteAddr, stripping the port.
// X-Forwarded-For is intentionally NOT trusted here to prevent rate-limit bypass
// via header spoofing. If the service runs behind a trusted reverse proxy, this
// function can be extended to validate the proxy's IP before trusting XFF.
func getClientIP(c echo.Context) string {
	ip := c.Request().RemoteAddr
	if idx := strings.LastIndex(ip, ":"); idx != -1 {
		ip = ip[:idx]
	}
	return ip
}

func (s *Server) rateLimitMiddleware(limit int, window time.Duration) echo.MiddlewareFunc {
	rl := s.rateLimiter
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ip := getClientIP(c)
			if !rl.allow(ip, limit, window) {
				return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
			}
			return next(c)
		}
	}
}

func getUserID(c echo.Context) int64 {
	id, _ := c.Get(userIDKey).(int64)
	return id
}

func getUserRole(c echo.Context) string {
	role, _ := c.Get(userRoleKey).(string)
	return role
}
