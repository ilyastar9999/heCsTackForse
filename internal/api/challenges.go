package api

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
	"github.com/ilyastar9999/heCsTackForse/internal/models"
	"github.com/ilyastar9999/heCsTackForse/internal/plugin"
)

func (s *Server) handleListChallenges(c echo.Context) error {
	userID := getUserID(c)
	rows, err := s.db.Query(
		`SELECT id, name, description, category, points, flag_type, deploy_type, deploy_backend, is_visible, connection_info, created_at,
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
			&ch.FlagType, &ch.DeployType, &ch.DeployBackend, &ch.IsVisible, &ch.ConnectionInfo, &ch.CreatedAt, &ch.SolveCount); err != nil {
			continue
		}
		// Apply the configured scoring plugin to the listed challenge.
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
		`SELECT id, name, description, category, points, flag_type, deploy_type, deploy_backend, image, is_visible, connection_info, created_at,
		(SELECT COUNT(*) FROM submissions WHERE challenge_id=challenges.id AND is_correct) as solve_count
		FROM challenges WHERE id=? AND is_visible`,
		id,
	).Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points, &ch.FlagType, &ch.DeployType, &ch.DeployBackend, &ch.Image, &ch.IsVisible, &ch.ConnectionInfo, &ch.CreatedAt, &ch.SolveCount)
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
	var challengeName string
	var points int
	var flagType string
	var solveCount int
	err = s.db.QueryRow("SELECT flag, name, points, flag_type, (SELECT COUNT(*) FROM submissions WHERE challenge_id=challenges.id AND is_correct) FROM challenges WHERE id=? AND is_visible", id).
		Scan(&correctFlag, &challengeName, &points, &flagType, &solveCount)
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

	// If not correct yet, check challenge_flags table for additional flags
	if !isCorrect {
		flagRows, _ := s.db.Query("SELECT content, type FROM challenge_flags WHERE challenge_id=?", id)
		if flagRows != nil {
			defer flagRows.Close()
			for flagRows.Next() {
				var fc, ft string
				if err := flagRows.Scan(&fc, &ft); err != nil {
					continue
				}
				altChecker, _ := plugin.Default.GetFlagChecker(ft)
				if altChecker == nil {
					altChecker, _ = plugin.Default.GetFlagChecker("exact")
				}
				if altChecker != nil && altChecker.Check(fc, req.Flag) {
					isCorrect = true
					break
				}
			}
		}
	}

	ip := getClientIP(c)

	if _, err := s.db.Exec(
		"INSERT INTO submissions (user_id, challenge_id, flag, is_correct, ip) VALUES (?, ?, ?, ?, ?)",
		userID, id, req.Flag, isCorrect, ip,
	); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error recording submission"})
	}

	if isCorrect {
		_, _ = s.db.Exec("UPDATE users SET score = score + ? WHERE id = ?", awardedPoints, userID)
		go s.notifySolve(userID, challengeName, awardedPoints, solveCount+1)
		return c.JSON(http.StatusOK, map[string]any{"correct": true, "points": awardedPoints})
	}
	return c.JSON(http.StatusOK, map[string]any{"correct": false})
}

func (s *Server) notifySolve(userID int64, challengeName string, points, solveNum int) {
	settings := s.loadNotifierSettings()
	if !settings.bool("notifier_send_notifications") || !settings.bool("notifier_send_solves") {
		return
	}

	maxSolveCount := settings.int("notifier_solve_count")
	if maxSolveCount > 0 && solveNum > maxSolveCount {
		return
	}

	solverName := "unknown"
	_ = s.db.QueryRow("SELECT username FROM users WHERE id=?", userID).Scan(&solverName)

	msgTemplate := settings.str("notifier_solve_msg")
	if strings.TrimSpace(msgTemplate) == "" {
		msgTemplate = "{solver} solved {challenge} ({solve_num} solve)"
	}
	msg := strings.NewReplacer(
		"{solver}", solverName,
		"{challenge}", challengeName,
		"{solve_num}", strconv.Itoa(solveNum),
		"{points}", strconv.Itoa(points),
	).Replace(msgTemplate)

	notifierName := strings.TrimSpace(settings.str("notifier_type"))
	n, err := plugin.Default.GetNotifier(notifierName)
	if err != nil || n == nil {
		notifiers := plugin.Default.NotifierNames()
		if len(notifiers) == 0 {
			return
		}
		n, err = plugin.Default.GetNotifier(notifiers[0])
		if err != nil || n == nil {
			return
		}
		notifierName = notifiers[0]
	}

	_ = n.Notify(plugin.Event{
		Type: "solve",
		Data: map[string]any{
			"solver":                       solverName,
			"challenge":                    challengeName,
			"points":                       points,
			"solve_num":                    solveNum,
			"message":                      msg,
			"notifier_plugin":              notifierName,
			"notifier_type":                settings.str("notifier_type"),
			"notifier_send_notifications":  settings.bool("notifier_send_notifications"),
			"notifier_send_solves":         settings.bool("notifier_send_solves"),
			"notifier_solve_msg":           msgTemplate,
			"notifier_solve_count":         settings.int("notifier_solve_count"),
			"notifier_slack_webhook_url":   settings.str("notifier_slack_webhook_url"),
			"notifier_discord_webhook_url": settings.str("notifier_discord_webhook_url"),
			"notifier_telegram_bot_token":  settings.str("notifier_telegram_bot_token"),
			"notifier_telegram_chat_id":    settings.str("notifier_telegram_chat_id"),
		},
	})
}

type notifierSettings map[string]string

func (s notifierSettings) str(key string) string {
	return strings.TrimSpace(s[key])
}

func (s notifierSettings) bool(key string) bool {
	v, err := strconv.ParseBool(strings.TrimSpace(s[key]))
	if err != nil {
		return false
	}
	return v
}

func (s notifierSettings) int(key string) int {
	v, err := strconv.Atoi(strings.TrimSpace(s[key]))
	if err != nil {
		return 0
	}
	return v
}

func (s *Server) loadNotifierSettings() notifierSettings {
	rows, err := s.db.Query(`SELECT key, value FROM ctf_settings WHERE key LIKE 'notifier_%'`)
	if err != nil {
		return notifierSettings{}
	}
	defer rows.Close()

	settings := notifierSettings{}
	for rows.Next() {
		var key, value string
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}
		settings[key] = value
	}

	if _, ok := settings["notifier_send_notifications"]; !ok {
		settings["notifier_send_notifications"] = "false"
	}
	if _, ok := settings["notifier_send_solves"]; !ok {
		settings["notifier_send_solves"] = "false"
	}
	if _, ok := settings["notifier_type"]; !ok {
		settings["notifier_type"] = ""
	}

	return settings
}

// scoreChallenge applies the configured scoring plugin to a challenge.
func (s *Server) scoreChallenge(ch *models.Challenge, solveCount int) int {
	scorerName := strings.TrimSpace(s.cfg.CTF.Scoring)
	if scorerName == "" {
		scorerName = "static"
	}
	scorer, err := plugin.Default.GetScorer(scorerName)
	if err != nil || scorer == nil {
		scorer, _ = plugin.Default.GetScorer("static")
	}
	if scorer == nil {
		return ch.Points
	}
	_ = scorer.Init(nil)
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
	rows, err := s.db.Query(`SELECT id, name, description, category, points, flag, flag_type, deploy_type, deploy_backend, deploy_config, image, vm_template, is_visible, connection_info, max_attempts, created_at FROM challenges ORDER BY id`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var challenges []models.Challenge
	for rows.Next() {
		var ch models.Challenge
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points, &ch.Flag, &ch.FlagType, &ch.DeployType, &ch.DeployBackend, &ch.DeployConfig, &ch.Image, &ch.VMTemplate, &ch.IsVisible, &ch.ConnectionInfo, &ch.MaxAttempts, &ch.CreatedAt); err != nil {
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
		`INSERT INTO challenges (name, description, category, points, flag, flag_type, deploy_type, deploy_backend, deploy_config, image, vm_template, is_visible, connection_info, max_attempts) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		ch.Name, ch.Description, ch.Category, ch.Points, ch.Flag, ch.FlagType, ch.DeployType, ch.DeployBackend, ch.DeployConfig, ch.Image, ch.VMTemplate, ch.IsVisible, ch.ConnectionInfo, ch.MaxAttempts,
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
		`UPDATE challenges SET name=?, description=?, category=?, points=?, flag=?, flag_type=?, deploy_type=?, deploy_backend=?, deploy_config=?, image=?, vm_template=?, is_visible=?, connection_info=?, max_attempts=? WHERE id=?`,
		ch.Name, ch.Description, ch.Category, ch.Points, ch.Flag, ch.FlagType, ch.DeployType, ch.DeployBackend, ch.DeployConfig, ch.Image, ch.VMTemplate, ch.IsVisible, ch.ConnectionInfo, ch.MaxAttempts, id,
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

// ─── Challenge Flags ─────────────────────────────────────────────────────────

func (s *Server) handleAdminListChallengeFlags(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	rows, err := s.db.Query(`SELECT id, challenge_id, content, type, data FROM challenge_flags WHERE challenge_id=? ORDER BY id`, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var flags []models.ChallengeFlag
	for rows.Next() {
		var f models.ChallengeFlag
		if err := rows.Scan(&f.ID, &f.ChallengeID, &f.Content, &f.Type, &f.Data); err != nil {
			continue
		}
		flags = append(flags, f)
	}
	if flags == nil {
		flags = []models.ChallengeFlag{}
	}
	return c.JSON(http.StatusOK, flags)
}

func (s *Server) handleAdminCreateChallengeFlag(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var req struct {
		Content string `json:"content"`
		Type    string `json:"type"`
		Data    string `json:"data"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Type == "" {
		req.Type = "exact"
	}
	fid, err := s.db.InsertGetID(
		`INSERT INTO challenge_flags (challenge_id, content, type, data) VALUES (?, ?, ?, ?)`,
		id, req.Content, req.Type, req.Data,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusCreated, models.ChallengeFlag{ID: fid, ChallengeID: id, Content: req.Content, Type: req.Type, Data: req.Data})
}

func (s *Server) handleAdminDeleteChallengeFlag(c echo.Context) error {
	fid, err := strconv.ParseInt(c.Param("fid"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if _, err := s.db.Exec(`DELETE FROM challenge_flags WHERE id=?`, fid); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

// ─── Challenge Hints ──────────────────────────────────────────────────────────

func (s *Server) handleAdminListChallengeHints(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	rows, err := s.db.Query(`SELECT id, challenge_id, content, cost, sort_order FROM challenge_hints WHERE challenge_id=? ORDER BY sort_order, id`, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var hints []models.ChallengeHint
	for rows.Next() {
		var h models.ChallengeHint
		if err := rows.Scan(&h.ID, &h.ChallengeID, &h.Content, &h.Cost, &h.SortOrder); err != nil {
			continue
		}
		hints = append(hints, h)
	}
	if hints == nil {
		hints = []models.ChallengeHint{}
	}
	return c.JSON(http.StatusOK, hints)
}

func (s *Server) handleAdminCreateChallengeHint(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var req struct {
		Content   string `json:"content"`
		Cost      int    `json:"cost"`
		SortOrder int    `json:"sort_order"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	hid, err := s.db.InsertGetID(
		`INSERT INTO challenge_hints (challenge_id, content, cost, sort_order) VALUES (?, ?, ?, ?)`,
		id, req.Content, req.Cost, req.SortOrder,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusCreated, models.ChallengeHint{ID: hid, ChallengeID: id, Content: req.Content, Cost: req.Cost, SortOrder: req.SortOrder})
}

func (s *Server) handleAdminDeleteChallengeHint(c echo.Context) error {
	hid, err := strconv.ParseInt(c.Param("hid"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if _, err := s.db.Exec(`DELETE FROM challenge_hints WHERE id=?`, hid); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

func (s *Server) handleListChallengeHints(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	rows, err := s.db.Query(`SELECT id, challenge_id, cost, sort_order FROM challenge_hints WHERE challenge_id=? ORDER BY sort_order, id`, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	type HintPublic struct {
		ID          int64 `json:"id"`
		ChallengeID int64 `json:"challenge_id"`
		Cost        int   `json:"cost"`
		SortOrder   int   `json:"sort_order"`
	}
	var hints []HintPublic
	for rows.Next() {
		var h HintPublic
		if err := rows.Scan(&h.ID, &h.ChallengeID, &h.Cost, &h.SortOrder); err != nil {
			continue
		}
		hints = append(hints, h)
	}
	if hints == nil {
		hints = []HintPublic{}
	}
	return c.JSON(http.StatusOK, hints)
}

// ─── Challenge Files ──────────────────────────────────────────────────────────

func (s *Server) handleAdminListChallengeFiles(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	rows, err := s.db.Query(`SELECT id, challenge_id, name, location, size FROM challenge_files WHERE challenge_id=? ORDER BY id`, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var files []models.ChallengeFile
	for rows.Next() {
		var f models.ChallengeFile
		if err := rows.Scan(&f.ID, &f.ChallengeID, &f.Name, &f.Location, &f.Size); err != nil {
			continue
		}
		files = append(files, f)
	}
	if files == nil {
		files = []models.ChallengeFile{}
	}
	return c.JSON(http.StatusOK, files)
}

func (s *Server) handleAdminUploadFile(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	file, err := c.FormFile("file")
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file required"})
	}
	src, err := file.Open()
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "cannot open file"})
	}
	defer src.Close()

	// Validate file extension against an allowlist of safe types
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowedExts := map[string]bool{
		".zip": true, ".tar": true, ".gz": true, ".7z": true, ".rar": true,
		".pdf": true, ".txt": true, ".md": true,
		".png": true, ".jpg": true, ".jpeg": true, ".gif": true, ".svg": true,
		".py": true, ".c": true, ".cpp": true, ".go": true, ".js": true, ".ts": true,
		".json": true, ".yaml": true, ".yml": true, ".xml": true,
		".pcap": true, ".pcapng": true, ".cap": true,
		".bin": true, ".elf": true, ".out": true,
	}
	if ext != "" && !allowedExts[ext] {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "file type not allowed"})
	}

	randBytes := make([]byte, 16)
	if _, err := crand.Read(randBytes); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}
	uniqueName := hex.EncodeToString(randBytes) + ext

	uploadsDir := filepath.Join(s.cfg.Server.StaticDir, "static", "uploads")
	if err := os.MkdirAll(uploadsDir, 0755); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "cannot create uploads dir"})
	}

	dst, err := os.Create(filepath.Join(uploadsDir, uniqueName))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "cannot create file"})
	}
	defer dst.Close()

	size, err := io.Copy(dst, src)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "cannot write file"})
	}

	location := "/static/uploads/" + uniqueName
	fid, err := s.db.InsertGetID(
		`INSERT INTO challenge_files (challenge_id, name, location, size) VALUES (?, ?, ?, ?)`,
		id, file.Filename, location, size,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusCreated, models.ChallengeFile{
		ID: fid, ChallengeID: id, Name: file.Filename, Location: location, Size: size,
	})
}

func (s *Server) handleAdminDeleteChallengeFile(c echo.Context) error {
	fid, err := strconv.ParseInt(c.Param("fid"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var location string
	_ = s.db.QueryRow(`SELECT location FROM challenge_files WHERE id=?`, fid).Scan(&location)
	if _, err := s.db.Exec(`DELETE FROM challenge_files WHERE id=?`, fid); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if location != "" {
		physPath := filepath.Join(s.cfg.Server.StaticDir, location)
		_ = os.Remove(physPath)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

func (s *Server) handleListChallengeFiles(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	rows, err := s.db.Query(`SELECT id, challenge_id, name, location, size FROM challenge_files WHERE challenge_id=? ORDER BY id`, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var files []models.ChallengeFile
	for rows.Next() {
		var f models.ChallengeFile
		if err := rows.Scan(&f.ID, &f.ChallengeID, &f.Name, &f.Location, &f.Size); err != nil {
			continue
		}
		files = append(files, f)
	}
	if files == nil {
		files = []models.ChallengeFile{}
	}
	return c.JSON(http.StatusOK, files)
}

func (s *Server) handleChallengeTypes(c echo.Context) error {
	return c.JSON(http.StatusOK, plugin.Default.ChallengeTypeNames())
}
