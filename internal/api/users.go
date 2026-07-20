package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) handleGetUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var u models.User
	err = s.db.QueryRow("SELECT id, username, role, score, affiliation, website, country, verified, created_at FROM users WHERE id=?", id).
		Scan(&u.ID, &u.Username, &u.Role, &u.Score, &u.Affiliation, &u.Website, &u.Country, &u.Verified, &u.CreatedAt)
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
	rows, err := s.db.Query("SELECT id, username, email, role, score, affiliation, website, country, banned, verified, hidden, language, created_at FROM users ORDER BY id")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.Score,
			&u.Affiliation, &u.Website, &u.Country, &u.Banned, &u.Verified, &u.Hidden, &u.Language, &u.CreatedAt); err != nil {
			continue
		}
		users = append(users, u)
	}
	if users == nil {
		users = []models.User{}
	}
	return c.JSON(http.StatusOK, users)
}

func (s *Server) handleAdminGetUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var u models.User
	err = s.db.QueryRow(
		"SELECT id, username, email, role, score, affiliation, website, country, banned, verified, hidden, language, created_at FROM users WHERE id=?", id,
	).Scan(&u.ID, &u.Username, &u.Email, &u.Role, &u.Score,
		&u.Affiliation, &u.Website, &u.Country, &u.Banned, &u.Verified, &u.Hidden, &u.Language, &u.CreatedAt)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	fieldRows, err := s.db.Query(
		`SELECT uf.id, uf.name, uf.field_type, uf.required, uf.public, uf.description, COALESCE(ufv.value, '')
 FROM user_fields uf LEFT JOIN user_field_values ufv ON uf.id=ufv.field_id AND ufv.user_id=?
 ORDER BY uf.id`, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer fieldRows.Close()

	type FieldWithValue struct {
		models.UserField
		Value string `json:"value"`
	}
	var fields []FieldWithValue
	for fieldRows.Next() {
		var f FieldWithValue
		if err := fieldRows.Scan(&f.ID, &f.Name, &f.FieldType, &f.Required, &f.Public, &f.Description, &f.Value); err != nil {
			continue
		}
		fields = append(fields, f)
	}
	if fields == nil {
		fields = []FieldWithValue{}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"user":   u,
		"fields": fields,
	})
}

func (s *Server) handleAdminUpdateUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var req struct {
		Username    string `json:"username"`
		Email       string `json:"email"`
		Role        string `json:"role"`
		Score       *int   `json:"score"`
		Banned      *bool  `json:"banned"`
		Verified    *bool  `json:"verified"`
		Hidden      *bool  `json:"hidden"`
		Language    string `json:"language"`
		Affiliation string `json:"affiliation"`
		Website     string `json:"website"`
		Country     string `json:"country"`
		Password    string `json:"password"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	if req.Username != "" {
		if _, err := s.db.Exec("UPDATE users SET username=? WHERE id=?", req.Username, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Email != "" {
		if _, err := s.db.Exec("UPDATE users SET email=? WHERE id=?", req.Email, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Role != "" {
		if _, err := s.db.Exec("UPDATE users SET role=? WHERE id=?", req.Role, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Score != nil {
		if _, err := s.db.Exec("UPDATE users SET score=? WHERE id=?", *req.Score, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Banned != nil {
		if _, err := s.db.Exec("UPDATE users SET banned=? WHERE id=?", *req.Banned, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Verified != nil {
		if _, err := s.db.Exec("UPDATE users SET verified=? WHERE id=?", *req.Verified, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Hidden != nil {
		if _, err := s.db.Exec("UPDATE users SET hidden=? WHERE id=?", *req.Hidden, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Language != "" {
		if _, err := s.db.Exec("UPDATE users SET language=? WHERE id=?", req.Language, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Affiliation != "" {
		if _, err := s.db.Exec("UPDATE users SET affiliation=? WHERE id=?", req.Affiliation, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Website != "" {
		if _, err := s.db.Exec("UPDATE users SET website=? WHERE id=?", req.Website, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Country != "" {
		if _, err := s.db.Exec("UPDATE users SET country=? WHERE id=?", req.Country, id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	if req.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
		}
		if _, err := s.db.Exec("UPDATE users SET password_hash=? WHERE id=?", string(hash), id); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}

	if req.Score != nil || req.Banned != nil || req.Hidden != nil {
		s.InvalidateScoreboardCache()
		s.InvalidateStatisticsCache()
	}

	return c.JSON(http.StatusOK, map[string]string{"message": "updated"})
}

// handleAdminDeleteUser permanently removes a user and all their submissions.
func (s *Server) handleAdminDeleteUser(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if _, err := s.db.Exec("DELETE FROM submissions WHERE user_id=?", id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if _, err := s.db.Exec("DELETE FROM team_members WHERE user_id=?", id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if _, err := s.db.Exec("DELETE FROM users WHERE id=?", id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	s.InvalidateScoreboardCache()
	s.InvalidateStatisticsCache()
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

// handleAdminResetScore sets a user's score to 0 and deletes all their submissions.
func (s *Server) handleAdminResetScore(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if _, err := s.db.Exec("DELETE FROM submissions WHERE user_id=?", id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if _, err := s.db.Exec("UPDATE users SET score=0 WHERE id=?", id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	s.InvalidateScoreboardCache()
	s.InvalidateStatisticsCache()
	return c.JSON(http.StatusOK, map[string]string{"message": "score reset"})
}

// ─── User Fields ─────────────────────────────────────────────────────────────

func (s *Server) handleAdminListUserFields(c echo.Context) error {
	rows, err := s.db.Query(`SELECT id, name, field_type, required, public, description FROM user_fields ORDER BY id`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var fields []models.UserField
	for rows.Next() {
		var f models.UserField
		if err := rows.Scan(&f.ID, &f.Name, &f.FieldType, &f.Required, &f.Public, &f.Description); err != nil {
			continue
		}
		fields = append(fields, f)
	}
	if fields == nil {
		fields = []models.UserField{}
	}
	return c.JSON(http.StatusOK, fields)
}

func (s *Server) handleAdminCreateUserField(c echo.Context) error {
	var req models.UserField
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name is required"})
	}
	if req.FieldType == "" {
		req.FieldType = "text"
	}
	id, err := s.db.InsertGetID(
		`INSERT INTO user_fields (name, field_type, required, public, description) VALUES (?, ?, ?, ?, ?)`,
		req.Name, req.FieldType, req.Required, req.Public, req.Description,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	req.ID = id
	return c.JSON(http.StatusCreated, req)
}

func (s *Server) handleAdminDeleteUserField(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if _, err := s.db.Exec(`DELETE FROM user_field_values WHERE field_id=?`, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if _, err := s.db.Exec(`DELETE FROM user_fields WHERE id=?`, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

func (s *Server) handleAdminSetUserFieldValue(c echo.Context) error {
	userID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid user id"})
	}
	fieldID, err := strconv.ParseInt(c.Param("fid"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid field id"})
	}
	var req struct {
		Value string `json:"value"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if _, err := s.db.Exec(
		s.db.InsertIgnore(`INSERT INTO user_field_values (user_id, field_id, value) VALUES (?, ?, ?)`),
		userID, fieldID, req.Value,
	); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if _, err := s.db.Exec(
		`UPDATE user_field_values SET value=? WHERE user_id=? AND field_id=?`,
		req.Value, userID, fieldID,
	); err != nil {
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
