package api

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) handleListChallenges(w http.ResponseWriter, r *http.Request) {
	userID := getUserID(r)
	rows, err := s.db.Query(
		`SELECT id, name, description, category, points, flag_type, deploy_type, deploy_backend, is_visible, created_at,
		(SELECT COUNT(*) FROM submissions WHERE challenge_id=challenges.id AND is_correct=1) as solve_count
		FROM challenges WHERE is_visible=1 ORDER BY category, points`)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var challenges []models.Challenge
	for rows.Next() {
		var c models.Challenge
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Category, &c.Points,
			&c.FlagType, &c.DeployType, &c.DeployBackend, &c.IsVisible, &c.CreatedAt, &c.SolveCount); err != nil {
			continue
		}
		var cnt int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct=1", userID, c.ID).Scan(&cnt)
		c.Solved = cnt > 0
		challenges = append(challenges, c)
	}
	if challenges == nil {
		challenges = []models.Challenge{}
	}
	jsonResponse(w, http.StatusOK, challenges)
}

func (s *Server) handleGetChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	userID := getUserID(r)
	var c models.Challenge
	err = s.db.QueryRow(
		`SELECT id, name, description, category, points, flag_type, deploy_type, deploy_backend, image, is_visible, created_at FROM challenges WHERE id=? AND is_visible=1`,
		id,
	).Scan(&c.ID, &c.Name, &c.Description, &c.Category, &c.Points, &c.FlagType, &c.DeployType, &c.DeployBackend, &c.Image, &c.IsVisible, &c.CreatedAt)
	if err == sql.ErrNoRows {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	_ = s.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE challenge_id=? AND is_correct=1", id).Scan(&c.SolveCount)
	var cnt int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct=1", userID, id).Scan(&cnt)
	c.Solved = cnt > 0
	jsonResponse(w, http.StatusOK, c)
}

func (s *Server) handleSubmitFlag(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	userID := getUserID(r)

	var req struct {
		Flag string `json:"flag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	var solvedCount int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct=1", userID, id).Scan(&solvedCount)
	if solvedCount > 0 {
		jsonError(w, "already solved", http.StatusConflict)
		return
	}

	var correctFlag string
	var points int
	var flagType string
	err = s.db.QueryRow("SELECT flag, points, flag_type FROM challenges WHERE id=? AND is_visible=1", id).Scan(&correctFlag, &points, &flagType)
	if err == sql.ErrNoRows {
		jsonError(w, "challenge not found", http.StatusNotFound)
		return
	}
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}

	ip := r.RemoteAddr
	var isCorrect bool
	if flagType == "regex" {
		re, err := regexp.Compile(correctFlag)
		isCorrect = err == nil && re.MatchString(req.Flag)
	} else {
		isCorrect = req.Flag == correctFlag
	}
	isCorrectInt := 0
	if isCorrect {
		isCorrectInt = 1
	}

	if _, err := s.db.Exec(
		"INSERT INTO submissions (user_id, challenge_id, flag, is_correct, ip) VALUES (?, ?, ?, ?, ?)",
		userID, id, req.Flag, isCorrectInt, ip,
	); err != nil {
		jsonError(w, "db error recording submission", http.StatusInternalServerError)
		return
	}

	if isCorrect {
		_, _ = s.db.Exec("UPDATE users SET score = score + ? WHERE id = ?", points, userID)
		jsonResponse(w, http.StatusOK, map[string]any{"correct": true, "points": points})
		return
	}
	jsonResponse(w, http.StatusOK, map[string]any{"correct": false})
}

func (s *Server) handleAdminListChallenges(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`SELECT id, name, description, category, points, flag, flag_type, deploy_type, deploy_backend, deploy_config, image, vm_template, is_visible, created_at FROM challenges ORDER BY id`)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var challenges []models.Challenge
	for rows.Next() {
		var c models.Challenge
		if err := rows.Scan(&c.ID, &c.Name, &c.Description, &c.Category, &c.Points, &c.Flag, &c.FlagType, &c.DeployType, &c.DeployBackend, &c.DeployConfig, &c.Image, &c.VMTemplate, &c.IsVisible, &c.CreatedAt); err != nil {
			continue
		}
		challenges = append(challenges, c)
	}
	if challenges == nil {
		challenges = []models.Challenge{}
	}
	jsonResponse(w, http.StatusOK, challenges)
}

func (s *Server) handleAdminCreateChallenge(w http.ResponseWriter, r *http.Request) {
	var c models.Challenge
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if c.DeployConfig == "" {
		c.DeployConfig = "{}"
	}
	result, err := s.db.Exec(
		`INSERT INTO challenges (name, description, category, points, flag, flag_type, deploy_type, deploy_backend, deploy_config, image, vm_template, is_visible) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		c.Name, c.Description, c.Category, c.Points, c.Flag, c.FlagType, c.DeployType, c.DeployBackend, c.DeployConfig, c.Image, c.VMTemplate, c.IsVisible,
	)
	if err != nil {
		jsonError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	id, _ := result.LastInsertId()
	c.ID = id
	jsonResponse(w, http.StatusCreated, c)
}

func (s *Server) handleAdminUpdateChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	var c models.Challenge
	if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if c.DeployConfig == "" {
		c.DeployConfig = "{}"
	}
	_, err = s.db.Exec(
		`UPDATE challenges SET name=?, description=?, category=?, points=?, flag=?, flag_type=?, deploy_type=?, deploy_backend=?, deploy_config=?, image=?, vm_template=?, is_visible=? WHERE id=?`,
		c.Name, c.Description, c.Category, c.Points, c.Flag, c.FlagType, c.DeployType, c.DeployBackend, c.DeployConfig, c.Image, c.VMTemplate, c.IsVisible, id,
	)
	if err != nil {
		jsonError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	c.ID = id
	jsonResponse(w, http.StatusOK, c)
}

func (s *Server) handleAdminDeleteChallenge(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	_, err = s.db.Exec("DELETE FROM challenges WHERE id=?", id)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "deleted"})
}
