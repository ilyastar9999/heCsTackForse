package api

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) setupRequired() bool {
	var count int
	if err := s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count); err != nil {
		return false
	}
	return count == 0
}

func (s *Server) handleSetupStatus(c echo.Context) error {
	return c.JSON(http.StatusOK, map[string]bool{"required": s.setupRequired()})
}

func (s *Server) handleSetup(c echo.Context) error {
	if !s.setupRequired() {
		return c.JSON(http.StatusConflict, map[string]string{"error": "setup is already complete"})
	}

	var req struct {
		SiteName         string `json:"site_name"`
		Mode             string `json:"mode"`
		TeamMode         bool   `json:"team_mode"`
		RegistrationOpen bool   `json:"registration_open"`
		Language         string `json:"language"`
		AdminUsername    string `json:"admin_username"`
		AdminEmail       string `json:"admin_email"`
		AdminPassword    string `json:"admin_password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	req.SiteName = strings.TrimSpace(req.SiteName)
	req.Mode = strings.TrimSpace(strings.ToLower(req.Mode))
	req.Language = strings.TrimSpace(strings.ToLower(req.Language))
	req.AdminUsername = strings.TrimSpace(req.AdminUsername)
	req.AdminEmail = strings.TrimSpace(req.AdminEmail)

	if req.SiteName == "" {
		req.SiteName = "heCsTackForse CTF"
	}
	if req.Mode == "" {
		req.Mode = "ctf"
	}
	if req.Mode != "ctf" && req.Mode != "ad" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "mode must be ctf or ad"})
	}
	if req.Language == "" {
		req.Language = "en"
	}
	if req.Language != "en" && req.Language != "ru" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "language must be en or ru"})
	}
	if req.AdminUsername == "" || req.AdminEmail == "" || req.AdminPassword == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "admin username, email, and password are required"})
	}
	if len(req.AdminPassword) < 8 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "admin password must be at least 8 characters"})
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	userID, err := s.db.InsertGetID(
		"INSERT INTO users (username, email, password_hash, role) VALUES (?, ?, ?, ?)",
		req.AdminUsername, req.AdminEmail, string(hash), "admin",
	)
	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "admin username or email already exists"})
	}

	settings := map[string]any{
		"ctf_name":          req.SiteName,
		"ctf_mode":          req.Mode,
		"language":          req.Language,
		"registration_open": req.RegistrationOpen,
		"team_mode":         req.TeamMode,
		"user_mode":         map[bool]string{true: "teams", false: "users"}[req.TeamMode],
		"setup_completed":   true,
	}
	for key, value := range settings {
		if _, err := s.db.Exec(
			`INSERT INTO ctf_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			key, configValueAsString(value),
		); err != nil {
			if _, err2 := s.db.Exec(`UPDATE ctf_settings SET value=? WHERE key=?`, configValueAsString(value), key); err2 != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
			}
		}
	}

	s.cfg.CTF.Name = req.SiteName
	s.cfg.CTF.Mode = req.Mode
	s.cfg.CTF.TeamMode = req.TeamMode
	s.cfg.CTF.RegistrationOpen = req.RegistrationOpen
	s.cfg.CTF.Language = req.Language

	user := &models.User{ID: userID, Username: req.AdminUsername, Email: req.AdminEmail, Role: "admin"}
	token, err := s.generateToken(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	setAuthCookie(c, token)
	return c.JSON(http.StatusCreated, map[string]any{"user": user})
}
