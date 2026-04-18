package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) handleGetUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var u models.User
	err = s.db.QueryRow("SELECT id, username, role, score, created_at FROM users WHERE id=?", id).
		Scan(&u.ID, &u.Username, &u.Role, &u.Score, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, u)
}

func (s *Server) handleCreateTeam(c echo.Context) error {
	userID := getUserID(c)
	var req struct {
		Name string `json:"name"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	inviteCode, err := generateInviteCode()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	teamID, err := s.db.InsertGetID("INSERT INTO teams (name, invite_code) VALUES (?, ?)", req.Name, inviteCode)
	if err != nil {
		return c.JSON(http.StatusConflict, map[string]string{"error": "team name already exists"})
	}
	_, err = s.db.Exec("INSERT INTO team_members (user_id, team_id) VALUES (?, ?)", userID, teamID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	team := models.Team{ID: teamID, Name: req.Name, InviteCode: inviteCode}
	return c.JSON(http.StatusCreated, team)
}

func (s *Server) handleJoinTeam(c echo.Context) error {
	userID := getUserID(c)
	var req struct {
		InviteCode string `json:"invite_code"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	var team models.Team
	err := s.db.QueryRow("SELECT id, name FROM teams WHERE invite_code=?", req.InviteCode).Scan(&team.ID, &team.Name)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "invalid invite code"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	_, err = s.db.Exec(s.db.InsertIgnore("INSERT INTO team_members (user_id, team_id) VALUES (?, ?)"), userID, team.ID)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, team)
}

func (s *Server) handleGetTeam(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var t models.Team
	err = s.db.QueryRow("SELECT id, name, score, created_at FROM teams WHERE id=?", id).
		Scan(&t.ID, &t.Name, &t.Score, &t.CreatedAt)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, t)
}

func (s *Server) handleAdminListUsers(c echo.Context) error {
	rows, err := s.db.Query("SELECT id, username, email, role, score, created_at FROM users ORDER BY id")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.Score, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	if users == nil {
		users = []models.User{}
	}
	return c.JSON(http.StatusOK, users)
}

func (s *Server) handleAdminUpdateUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var req struct {
		Role  string `json:"role"`
		Score int    `json:"score"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	_, err = s.db.Exec("UPDATE users SET role=?, score=? WHERE id=?", req.Role, req.Score, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "updated"})
}

func generateInviteCode() (string, error) {
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
