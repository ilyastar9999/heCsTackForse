package api

import (
	"net/http"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) handleRegister(c echo.Context) error {
	if !s.cfg.CTF.RegistrationOpen {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "registration is closed"})
	}
	var req struct {
		Username string `json:"username"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Username == "" || req.Email == "" || req.Password == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "username, email, and password are required"})
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	role := "user"
	// Make first user admin
	var count int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM users").Scan(&count)
	if count == 0 {
		role = "admin"
	}
	id, err := s.db.InsertGetID(
		"INSERT INTO users (username, email, password_hash, role) VALUES (?, ?, ?, ?)",
		req.Username, req.Email, string(hash), role,
	)
	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "username or email already exists"})
	}
	user := &models.User{
		ID:       id,
		Username: req.Username,
		Email:    req.Email,
		Role:     role,
	}
	token, err := s.generateToken(user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	setAuthCookie(c, token)
	return c.JSON(http.StatusCreated, map[string]any{
		"token": token,
		"user":  user,
	})
}

func (s *Server) handleLogin(c echo.Context) error {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	var user models.User
	err := s.db.QueryRow(
		"SELECT id, username, email, password_hash, role, score, created_at FROM users WHERE username = ?",
		req.Username,
	).Scan(&user.ID, &user.Username, &user.Email, &user.PasswordHash, &user.Role, &user.Score, &user.CreatedAt)
	if err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "invalid credentials"})
	}
	token, err := s.generateToken(&user)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	setAuthCookie(c, token)
	return c.JSON(http.StatusOK, map[string]any{
		"token": token,
		"user":  user,
	})
}

func (s *Server) handleMe(c echo.Context) error {
	userID := getUserID(c)
	var user models.User
	err := s.db.QueryRow(
		"SELECT id, username, email, role, score, affiliation, website, country, banned, created_at FROM users WHERE id = ?",
		userID,
	).Scan(&user.ID, &user.Username, &user.Email, &user.Role, &user.Score,
		&user.Affiliation, &user.Website, &user.Country, &user.Banned, &user.CreatedAt)
	if err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	return c.JSON(http.StatusOK, user)
}

func (s *Server) handleLogout(c echo.Context) error {
	http.SetCookie(c.Response(), &http.Cookie{
		Name:     "token",
		Value:    "",
		Path:     "/",
		MaxAge:   -1,
		HttpOnly: true,
		Secure:   c.Request().TLS != nil,
	})
	return c.JSON(http.StatusOK, map[string]string{"message": "logged out"})
}

// handleUpdateMe allows an authenticated user to update their own profile fields.
func (s *Server) handleUpdateMe(c echo.Context) error {
	userID := getUserID(c)
	var req struct {
		Affiliation string `json:"affiliation"`
		Website     string `json:"website"`
		Country     string `json:"country"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if _, err := s.db.Exec(
		"UPDATE users SET affiliation=?, website=?, country=? WHERE id=?",
		req.Affiliation, req.Website, req.Country, userID,
	); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "profile updated"})
}

// handleChangePassword lets an authenticated user change their own password.
func (s *Server) handleChangePassword(c echo.Context) error {
	userID := getUserID(c)
	var req struct {
		OldPassword string `json:"old_password"`
		NewPassword string `json:"new_password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.NewPassword == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "new password required"})
	}

	var hash string
	if err := s.db.QueryRow("SELECT password_hash FROM users WHERE id=?", userID).Scan(&hash); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "user not found"})
	}
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(req.OldPassword)); err != nil {
		return c.JSON(http.StatusUnauthorized, map[string]string{"error": "current password is incorrect"})
	}
	newHash, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	if _, err := s.db.Exec("UPDATE users SET password_hash=? WHERE id=?", string(newHash), userID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "password changed"})
}

func (s *Server) generateToken(user *models.User) (string, error) {
	claims := jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.cfg.Server.SecretKey))
}

func setAuthCookie(c echo.Context, token string) {
	http.SetCookie(c.Response(), &http.Cookie{
		Name:     "token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   c.Request().TLS != nil,
		SameSite: http.SameSiteLaxMode,
	})
}
