package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
	"github.com/ilyastar9999/heCsTackForse/internal/models"
	"github.com/ilyastar9999/heCsTackForse/internal/plugin"
)

func (s *Server) handleListChallenges(c echo.Context) error {
	userID := getUserID(c)
	rows, err := s.db.Query(
		`SELECT id, name, description, category, points, flag_type, deploy_type, deploy_backend, is_visible, created_at,
		(SELECT COUNT(*) FROM submissions WHERE challenge_id=challenges.id AND is_correct) as solve_count
		FROM challenges WHERE is_visible ORDER BY category, points`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	var challenges []models.Challenge
	for rows.Next() {
		var ch models.Challenge
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points,
			&ch.FlagType, &ch.DeployType, &ch.DeployBackend, &ch.IsVisible, &ch.CreatedAt, &ch.SolveCount); err != nil {
			continue
		}
		// Apply dynamic scoring if configured
		ch.Points = s.scoreChallenge(&ch, ch.SolveCount)
		var cnt int
		_ = s.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct", userID, ch.ID).Scan(&cnt)
		ch.Solved = cnt > 0
		challenges = append(challenges, ch)
	}
	if challenges == nil {
		challenges = []models.Challenge{}
	}
	return c.JSON(http.StatusOK, challenges)
}

func (s *Server) handleGetChallenge(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)
	var ch models.Challenge
	err = s.db.QueryRow(
		`SELECT id, name, description, category, points, flag_type, deploy_type, deploy_backend, image, is_visible, created_at,
		(SELECT COUNT(*) FROM submissions WHERE challenge_id=challenges.id AND is_correct) as solve_count
		FROM challenges WHERE id=? AND is_visible`,
		id,
	).Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points, &ch.FlagType, &ch.DeployType, &ch.DeployBackend, &ch.Image, &ch.IsVisible, &ch.CreatedAt, &ch.SolveCount)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	ch.Points = s.scoreChallenge(&ch, ch.SolveCount)
	var cnt int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct", userID, id).Scan(&cnt)
	ch.Solved = cnt > 0
	return c.JSON(http.StatusOK, ch)
}

func (s *Server) handleSubmitFlag(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)

	var req struct {
		Flag string `json:"flag"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	var solvedCount int
	_ = s.db.QueryRow("SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct", userID, id).Scan(&solvedCount)
	if solvedCount > 0 {
		return c.JSON(http.StatusConflict, map[string]string{"error": "already solved"})
	}

	var correctFlag string
	var points int
	var flagType string
	var solveCount int
	err = s.db.QueryRow("SELECT flag, points, flag_type, (SELECT COUNT(*) FROM submissions WHERE challenge_id=challenges.id AND is_correct) FROM challenges WHERE id=? AND is_visible", id).
		Scan(&correctFlag, &points, &flagType, &solveCount)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	// Calculate score using configured scorer
	ch := &models.Challenge{Points: points}
	awardedPoints := s.scoreChallenge(ch, solveCount)

	checker, _ := plugin.Default.GetFlagChecker(flagType)
	if checker == nil {
		checker, _ = plugin.Default.GetFlagChecker("exact")
	}
	isCorrect := checker.Check(correctFlag, req.Flag)
	ip := getClientIP(c)

	if _, err := s.db.Exec(
		"INSERT INTO submissions (user_id, challenge_id, flag, is_correct, ip) VALUES (?, ?, ?, ?, ?)",
		userID, id, req.Flag, isCorrect, ip,
	); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error recording submission"})
	}

	if isCorrect {
		_, _ = s.db.Exec("UPDATE users SET score = score + ? WHERE id = ?", awardedPoints, userID)
		return c.JSON(http.StatusOK, map[string]any{"correct": true, "points": awardedPoints})
	}
	return c.JSON(http.StatusOK, map[string]any{"correct": false})
}

// scoreChallenge applies the configured scoring plugin to a challenge.
func (s *Server) scoreChallenge(ch *models.Challenge, solveCount int) int {
	if s.cfg.CTF.Scoring == "dynamic" {
		scorer := &plugin.DynamicScorer{}
		_ = scorer.Init(nil)
		return scorer.CalculateScore(ch, solveCount)
	}
	scorer := &plugin.StaticScorer{}
	return scorer.CalculateScore(ch, solveCount)
}

// ─── Instance management ──────────────────────────────────────────────────────

func (s *Server) handleGetInstance(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)
	var inst models.Instance
	err = s.db.QueryRow(
		`SELECT id, challenge_id, user_id, team_id, instance_type, backend, instance_id, connection_info, created_at, expires_at, status
		 FROM instances WHERE challenge_id=? AND user_id=? ORDER BY created_at DESC LIMIT 1`,
		challengeID, userID,
	).Scan(&inst.ID, &inst.ChallengeID, &inst.UserID, &inst.TeamID, &inst.InstanceType, &inst.Backend,
		&inst.InstanceID, &inst.ConnectionInfo, &inst.CreatedAt, &inst.ExpiresAt, &inst.Status)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no instance"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, inst)
}

func (s *Server) handleStartInstance(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)

	// Load challenge
	var ch models.Challenge
	err = s.db.QueryRow(
		`SELECT id, deploy_type, deploy_backend, deploy_config, image, vm_template FROM challenges WHERE id=? AND is_visible`,
		challengeID,
	).Scan(&ch.ID, &ch.DeployType, &ch.DeployBackend, &ch.DeployConfig, &ch.Image, &ch.VMTemplate)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if ch.DeployType == "no_deploy" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "this challenge has no deployable instance"})
	}

	// Check for existing running instance
	var existingID int64
	_ = s.db.QueryRow(
		`SELECT id FROM instances WHERE challenge_id=? AND user_id=? AND status='running' LIMIT 1`,
		challengeID, userID,
	).Scan(&existingID)
	if existingID > 0 {
		return c.JSON(http.StatusConflict, map[string]string{"error": "instance already running"})
	}

	back, err := s.deployer.Get(ch.DeployBackend)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "deployer backend not available: " + err.Error()})
	}

	var deployConfig map[string]any
	if ch.DeployConfig != "" && ch.DeployConfig != "{}" {
		_ = json.Unmarshal([]byte(ch.DeployConfig), &deployConfig)
	}

	req := deployer.DeployRequest{
		ChallengeID:  ch.ID,
		UserID:       &userID,
		Image:        ch.Image,
		VMTemplate:   ch.VMTemplate,
		DeployConfig: deployConfig,
		DeployType:   ch.DeployType,
		Backend:      ch.DeployBackend,
		InstanceTTL:  s.cfg.Deployer.InstanceTTL,
	}

	inst, err := back.Deploy(context.Background(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "deploy failed: " + err.Error()})
	}

	// Persist instance
	connInfo := inst.ConnectionInfo
	if connInfo == "" {
		connInfo = "{}"
	}
	newID, dbErr := s.db.InsertGetID(
		`INSERT INTO instances (challenge_id, user_id, instance_type, backend, instance_id, connection_info, expires_at, status) VALUES (?,?,?,?,?,?,?,?)`,
		challengeID, userID, ch.DeployType, ch.DeployBackend, inst.InstanceID, connInfo, inst.ExpiresAt, "running",
	)
	if dbErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error saving instance"})
	}
	inst.ID = newID
	return c.JSON(http.StatusCreated, inst)
}

func (s *Server) handleStopInstance(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)

	var inst models.Instance
	err = s.db.QueryRow(
		`SELECT id, backend, instance_id FROM instances WHERE challenge_id=? AND user_id=? AND status='running' ORDER BY created_at DESC LIMIT 1`,
		challengeID, userID,
	).Scan(&inst.ID, &inst.Backend, &inst.InstanceID)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no running instance"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	back, err := s.deployer.Get(inst.Backend)
	if err == nil {
		_ = back.Destroy(context.Background(), inst.InstanceID)
	}

	_, _ = s.db.Exec(`UPDATE instances SET status='stopped' WHERE id=?`, inst.ID)
	return c.JSON(http.StatusOK, map[string]string{"message": "instance stopped"})
}

func (s *Server) handleAdminListChallenges(c echo.Context) error {
	rows, err := s.db.Query(`SELECT id, name, description, category, points, flag, flag_type, deploy_type, deploy_backend, deploy_config, image, vm_template, is_visible, created_at FROM challenges ORDER BY id`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var challenges []models.Challenge
	for rows.Next() {
		var ch models.Challenge
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points, &ch.Flag, &ch.FlagType, &ch.DeployType, &ch.DeployBackend, &ch.DeployConfig, &ch.Image, &ch.VMTemplate, &ch.IsVisible, &ch.CreatedAt); err != nil {
			continue
		}
		challenges = append(challenges, ch)
	}
	if challenges == nil {
		challenges = []models.Challenge{}
	}
	return c.JSON(http.StatusOK, challenges)
}

func (s *Server) handleAdminCreateChallenge(c echo.Context) error {
	var ch models.Challenge
	if err := c.Bind(&ch); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if ch.DeployConfig == "" {
		ch.DeployConfig = "{}"
	}
	id, err := s.db.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, flag_type, deploy_type, deploy_backend, deploy_config, image, vm_template, is_visible) VALUES (?,?,?,?,?,?,?,?,?,?,?,?)`,
		ch.Name, ch.Description, ch.Category, ch.Points, ch.Flag, ch.FlagType, ch.DeployType, ch.DeployBackend, ch.DeployConfig, ch.Image, ch.VMTemplate, ch.IsVisible,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error: " + err.Error()})
	}
	ch.ID = id
	return c.JSON(http.StatusCreated, ch)
}

func (s *Server) handleAdminUpdateChallenge(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var ch models.Challenge
	if err := c.Bind(&ch); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if ch.DeployConfig == "" {
		ch.DeployConfig = "{}"
	}
	_, err = s.db.Exec(
		`UPDATE challenges SET name=?, description=?, category=?, points=?, flag=?, flag_type=?, deploy_type=?, deploy_backend=?, deploy_config=?, image=?, vm_template=?, is_visible=? WHERE id=?`,
		ch.Name, ch.Description, ch.Category, ch.Points, ch.Flag, ch.FlagType, ch.DeployType, ch.DeployBackend, ch.DeployConfig, ch.Image, ch.VMTemplate, ch.IsVisible, id,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error: " + err.Error()})
	}
	ch.ID = id
	return c.JSON(http.StatusOK, ch)
}

func (s *Server) handleAdminDeleteChallenge(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	_, err = s.db.Exec("DELETE FROM challenges WHERE id=?", id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}
