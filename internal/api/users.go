package api

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) handleGetUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var u models.User
	err = s.db.QueryRow("SELECT id, username, role, score, created_at FROM users WHERE id=?", id).
		Scan(&u.ID, &u.Username, &u.Role, &u.Score, &u.CreatedAt)
	if err == sql.ErrNoRows {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusOK, u)
}

func (s *Server) handleCreateTeam(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Name == "" {
		jsonError(w, "name is required", http.StatusBadRequest)
		return
	}
	inviteCode, err := generateInviteCode()
	if err != nil {
		jsonError(w, "internal error", http.StatusInternalServerError)
		return
	}
	result, err := s.db.Exec("INSERT INTO teams (name, invite_code) VALUES (?, ?)", req.Name, inviteCode)
	if err != nil {
		jsonError(w, "team name already exists", http.StatusConflict)
		return
	}
	teamID, _ := result.LastInsertId()
	_, err = s.db.Exec("INSERT INTO team_members (user_id, team_id) VALUES (?, ?)", userID, teamID)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	team := models.Team{ID: teamID, Name: req.Name, InviteCode: inviteCode}
	jsonResponse(w, http.StatusCreated, team)
}

func (s *Server) handleJoinTeam(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	var req struct {
		InviteCode string `json:"invite_code"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	var team models.Team
	err := s.db.QueryRow("SELECT id, name FROM teams WHERE invite_code=?", req.InviteCode).Scan(&team.ID, &team.Name)
	if err == sql.ErrNoRows {
		jsonError(w, "invalid invite code", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	_, err = s.db.Exec("INSERT OR IGNORE INTO team_members (user_id, team_id) VALUES (?, ?)", userID, team.ID)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusOK, team)
}

func (s *Server) handleGetTeam(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var t models.Team
	err = s.db.QueryRow("SELECT id, name, score, created_at FROM teams WHERE id=?", id).
		Scan(&t.ID, &t.Name, &t.Score, &t.CreatedAt)
	if err == sql.ErrNoRows {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusOK, t)
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, username, email, role, score, created_at FROM users ORDER BY id")
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
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
	jsonResponse(w, http.StatusOK, users)
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var req struct {
		Role  string `json:"role"`
		Score int    `json:"score"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	_, err = s.db.Exec("UPDATE users SET role=?, score=? WHERE id=?", req.Role, req.Score, id)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "updated"})
}

func generateInviteCode() (string, error) {
	b := make([]byte, 12)
	_, err := rand.Read(b)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
