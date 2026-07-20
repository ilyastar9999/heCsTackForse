package api

import (
	"context"
	crand "crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
	"github.com/ilyastar9999/heCsTackForse/internal/models"
	"github.com/ilyastar9999/heCsTackForse/internal/plugin"
)

func (s *Server) handleListChallenges(c echo.Context) error {
	userID := getUserID(c)
	rows, err := s.db.Query(
		`SELECT c.id, c.name, c.description, c.category, c.points, c.challenge_type, c.flag_type, c.checker_config, c.deploy_type, c.deploy_backend,
		        c.is_visible, c.connection_info, c.created_at,
		        COALESCE(sc.solve_count, 0) AS solve_count,
		        CASE WHEN us.challenge_id IS NULL THEN 0 ELSE 1 END AS solved
		   FROM challenges c
		   LEFT JOIN (
		       SELECT challenge_id, COUNT(*) AS solve_count
		         FROM submissions
		        WHERE is_correct
		        GROUP BY challenge_id
		   ) sc ON sc.challenge_id = c.id
		   LEFT JOIN (
		       SELECT DISTINCT challenge_id
		         FROM submissions
		        WHERE user_id=? AND is_correct
		   ) us ON us.challenge_id = c.id
		  WHERE c.is_visible
		  ORDER BY c.category, c.points, c.id`,
		userID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	var challenges []models.Challenge
	for rows.Next() {
		var ch models.Challenge
		var solved int
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points,
			&ch.ChallengeType, &ch.FlagType, &ch.CheckerConfig, &ch.DeployType, &ch.DeployBackend, &ch.IsVisible, &ch.ConnectionInfo, &ch.CreatedAt,
			&ch.SolveCount, &solved); err != nil {
			continue
		}
		// Apply the configured scoring plugin to the listed challenge.
		ch.Points = s.scoreChallenge(&ch, ch.SolveCount)
		ch.Solved = solved > 0
		challenges = append(challenges, ch)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
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
	var solved int
	err = s.db.QueryRow(
		`SELECT c.id, c.name, c.description, c.category, c.points, c.challenge_type, c.flag_type, c.checker_config, c.deploy_type, c.deploy_backend,
		        c.image, c.is_visible, c.connection_info, c.created_at,
		        COALESCE(sc.solve_count, 0) AS solve_count,
		        CASE WHEN us.challenge_id IS NULL THEN 0 ELSE 1 END AS solved
		   FROM challenges c
		   LEFT JOIN (
		       SELECT challenge_id, COUNT(*) AS solve_count
		         FROM submissions
		        WHERE is_correct
		        GROUP BY challenge_id
		   ) sc ON sc.challenge_id = c.id
		   LEFT JOIN (
		       SELECT DISTINCT challenge_id
		         FROM submissions
		        WHERE user_id=? AND is_correct
		   ) us ON us.challenge_id = c.id
		  WHERE c.id=? AND c.is_visible`,
		userID, id,
	).Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points, &ch.ChallengeType, &ch.FlagType, &ch.CheckerConfig, &ch.DeployType, &ch.DeployBackend, &ch.Image, &ch.IsVisible, &ch.ConnectionInfo, &ch.CreatedAt, &ch.SolveCount, &solved)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	ch.Points = s.scoreChallenge(&ch, ch.SolveCount)
	ch.Solved = solved > 0
	return c.JSON(http.StatusOK, ch)
}

func (s *Server) handleSubmitFlag(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)
	teamID := int64(0)
	if s.cfg.CTF.TeamMode {
		teamID = s.getTeamIDForUser(userID)
	}

	var req struct {
		Flag string `json:"flag"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	req.Flag = strings.TrimSpace(req.Flag)
	if req.Flag == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "flag is required"})
	}
	if len(req.Flag) > 1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "flag too long"})
	}

	var correctFlag string
	var challengeName string
	var points int
	var challengeType string
	var flagType string
	var checkerConfig string
	var deployType string
	var solveCount int
	var maxAttempts int
	err = s.db.QueryRow("SELECT flag, name, points, challenge_type, flag_type, checker_config, deploy_type, max_attempts, (SELECT COUNT(*) FROM submissions WHERE challenge_id=challenges.id AND is_correct) FROM challenges WHERE id=? AND is_visible", id).
		Scan(&correctFlag, &challengeName, &points, &challengeType, &flagType, &checkerConfig, &deployType, &maxAttempts, &solveCount)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	if maxAttempts > 0 {
		var attempts int
		_ = s.db.QueryRow(
			"SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct=FALSE",
			userID, id,
		).Scan(&attempts)
		if attempts >= maxAttempts {
			return c.JSON(http.StatusTooManyRequests, map[string]string{"error": "maximum attempts exceeded"})
		}
	}

	// Calculate score using configured scorer
	ch := &models.Challenge{Points: points}
	defaultAwardedPoints := s.scoreChallenge(ch, solveCount)
	submission := submissionOutcome{
		BucketKey:     "",
		AwardedPoints: defaultAwardedPoints,
	}
	switch challengeType {
	case "attack_defence_attack", "attack_defence_defense":
		return c.JSON(http.StatusConflict, map[string]string{"error": "this challenge type is scored by the attack-defense engine and does not accept direct flag submissions"})
	case "", "static", "pentest":
		submission = s.matchStandardOrPentestSubmission(id, challengeType, correctFlag, flagType, checkerConfig, req.Flag, defaultAwardedPoints)
	case "dynamic_deploy":
		expectedFlag, found := s.lookupDynamicFlag(id, userID, deployType)
		if !found {
			return c.JSON(http.StatusConflict, map[string]string{"error": "start your challenge instance before submitting the dynamic flag"})
		}
		checker, _ := plugin.Default.GetFlagChecker(flagType)
		if checker == nil {
			checker, _ = plugin.Default.GetFlagChecker("exact")
		}
		submission.IsCorrect = checker.Check(expectedFlag, req.Flag)
	default:
		ext, extErr := plugin.Default.GetChallengeType(challengeType)
		if extErr != nil || ext == nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "challenge type is not registered"})
		}
		payload := checkerConfig
		if strings.TrimSpace(payload) == "" {
			payload = "{}"
		}
		submission.IsCorrect = ext.Verify(payload, req.Flag)
	}

	ip := getClientIP(c)
	var dbTeamID any
	if teamID > 0 {
		dbTeamID = teamID
	}

	err = s.db.Transaction(func(tx *sql.Tx) error {
		if challengeType != "pentest" {
			var solvedCount int
			solveQuery := "SELECT COUNT(*) FROM submissions WHERE challenge_id=? AND is_correct=TRUE"
			solveArgs := []any{id}
			if s.cfg.CTF.TeamMode && teamID > 0 {
				solveQuery += " AND team_id=?"
				solveArgs = append(solveArgs, teamID)
			} else {
				solveQuery += " AND user_id=?"
				solveArgs = append(solveArgs, userID)
			}
			if err := tx.QueryRow(s.db.Rewrite(solveQuery), solveArgs...).Scan(&solvedCount); err != nil {
				return err
			}
			if solvedCount > 0 {
				return fmt.Errorf("already solved")
			}
		} else if submission.IsCorrect && submission.BucketKey != "" {
			var alreadySolvedCount int
			bucketQuery := "SELECT COUNT(*) FROM submissions WHERE challenge_id=? AND bucket_key=? AND is_correct=TRUE"
			bucketArgs := []any{id, submission.BucketKey}
			if s.cfg.CTF.TeamMode && teamID > 0 {
				bucketQuery += " AND team_id=?"
				bucketArgs = append(bucketArgs, teamID)
			} else {
				bucketQuery += " AND user_id=?"
				bucketArgs = append(bucketArgs, userID)
			}
			if err := tx.QueryRow(s.db.Rewrite(bucketQuery), bucketArgs...).Scan(&alreadySolvedCount); err != nil {
				return err
			}
			if alreadySolvedCount > 0 {
				return fmt.Errorf("this pentest bucket is already solved")
			}
		}
		if _, err := tx.Exec(
			s.db.Rewrite("INSERT INTO submissions (user_id, team_id, challenge_id, bucket_key, awarded_points, flag, is_correct, ip) VALUES (?, ?, ?, ?, ?, ?, ?, ?)"),
			userID, dbTeamID, id, submission.BucketKey, submission.AwardedPoints, req.Flag, submission.IsCorrect, ip,
		); err != nil {
			return err
		}
		if submission.IsCorrect {
			if _, err := tx.Exec(s.db.Rewrite("UPDATE users SET score = score + ? WHERE id = ?"), submission.AwardedPoints, userID); err != nil {
				return err
			}
			if teamID > 0 {
				if _, err := tx.Exec(s.db.Rewrite("UPDATE teams SET score = score + ? WHERE id = ?"), submission.AwardedPoints, teamID); err != nil {
					return err
				}
			}
		}
		return nil
	})
	if err != nil {
		if err.Error() == "already solved" {
			return c.JSON(http.StatusConflict, map[string]string{"error": "already solved"})
		}
		if err.Error() == "this pentest bucket is already solved" {
			return c.JSON(http.StatusConflict, map[string]string{"error": "this pentest bucket is already solved"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	if submission.IsCorrect {
		go s.InvalidateScoreboardCache()
		go s.InvalidateStatisticsCache()
		go s.notifySolve(userID, challengeName, submission.AwardedPoints, solveCount+1)
		return c.JSON(http.StatusOK, map[string]any{
			"correct":      true,
			"points":       submission.AwardedPoints,
			"bucket_key":   submission.BucketKey,
			"challenge_id": id,
		})
	}
	return c.JSON(http.StatusOK, map[string]any{"correct": false})
}

func (s *Server) handleSubmitAnyFlag(c echo.Context) error {
	userID := getUserID(c)
	teamID := int64(0)
	if s.cfg.CTF.TeamMode {
		teamID = s.getTeamIDForUser(userID)
	}

	var req struct {
		Flag string `json:"flag"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	req.Flag = strings.TrimSpace(req.Flag)
	if req.Flag == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "flag is required"})
	}
	if len(req.Flag) > 1024 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "flag too long"})
	}

	rows, err := s.db.Query(`
		SELECT id, flag, name, points, challenge_type, flag_type, checker_config, deploy_type, max_attempts,
		       (SELECT COUNT(*) FROM submissions WHERE challenge_id=challenges.id AND is_correct)
		  FROM challenges
		 WHERE is_visible
		 ORDER BY id`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	type submitTarget struct {
		id            int64
		correctFlag   string
		name          string
		points        int
		challengeType string
		flagType      string
		checkerConfig string
		deployType    string
		maxAttempts   int
		solveCount    int
	}

	for rows.Next() {
		var target submitTarget
		if err := rows.Scan(
			&target.id,
			&target.correctFlag,
			&target.name,
			&target.points,
			&target.challengeType,
			&target.flagType,
			&target.checkerConfig,
			&target.deployType,
			&target.maxAttempts,
			&target.solveCount,
		); err != nil {
			continue
		}

		if target.challengeType == "attack_defence_attack" || target.challengeType == "attack_defence_defense" {
			continue
		}

		defaultAwardedPoints := s.scoreChallenge(&models.Challenge{Points: target.points}, target.solveCount)
		submission := submissionOutcome{AwardedPoints: defaultAwardedPoints}
		switch target.challengeType {
		case "", "static", "pentest":
			submission = s.matchStandardOrPentestSubmission(target.id, target.challengeType, target.correctFlag, target.flagType, target.checkerConfig, req.Flag, defaultAwardedPoints)
		case "dynamic_deploy":
			expectedFlag, found := s.lookupDynamicFlag(target.id, userID, target.deployType)
			if !found {
				continue
			}
			checker, _ := plugin.Default.GetFlagChecker(target.flagType)
			if checker == nil {
				checker, _ = plugin.Default.GetFlagChecker("exact")
			}
			submission.IsCorrect = checker != nil && checker.Check(expectedFlag, req.Flag)
		default:
			ext, extErr := plugin.Default.GetChallengeType(target.challengeType)
			if extErr != nil || ext == nil {
				continue
			}
			payload := target.checkerConfig
			if strings.TrimSpace(payload) == "" {
				payload = "{}"
			}
			submission.IsCorrect = ext.Verify(payload, req.Flag)
		}
		if !submission.IsCorrect {
			continue
		}

		if target.maxAttempts > 0 {
			var attempts int
			_ = s.db.QueryRow(
				"SELECT COUNT(*) FROM submissions WHERE user_id=? AND challenge_id=? AND is_correct=FALSE",
				userID, target.id,
			).Scan(&attempts)
			if attempts >= target.maxAttempts {
				return c.JSON(http.StatusTooManyRequests, map[string]any{
					"error":          "maximum attempts exceeded",
					"challenge_id":   target.id,
					"challenge_name": target.name,
				})
			}
		}

		if target.challengeType != "pentest" {
			solvedCount, err := s.countSolvedByOwner(target.id, userID, teamID)
			if err == nil && solvedCount > 0 {
				return c.JSON(http.StatusConflict, map[string]any{
					"error":          "already solved",
					"challenge_id":   target.id,
					"challenge_name": target.name,
				})
			}
		} else if submission.BucketKey != "" {
			alreadySolved, err := s.bucketAlreadySolved(target.id, submission.BucketKey, userID, teamID)
			if err == nil && alreadySolved {
				return c.JSON(http.StatusConflict, map[string]any{
					"error":          "this pentest bucket is already solved",
					"challenge_id":   target.id,
					"challenge_name": target.name,
					"bucket_key":     submission.BucketKey,
				})
			}
		}

		ip := getClientIP(c)
		var dbTeamID any
		if teamID > 0 {
			dbTeamID = teamID
		}
		if _, err := s.db.Exec(
			"INSERT INTO submissions (user_id, team_id, challenge_id, bucket_key, awarded_points, flag, is_correct, ip) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			userID, dbTeamID, target.id, submission.BucketKey, submission.AwardedPoints, req.Flag, true, ip,
		); err != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error recording submission"})
		}
		_, _ = s.db.Exec("UPDATE users SET score = score + ? WHERE id = ?", submission.AwardedPoints, userID)
		if teamID > 0 {
			_, _ = s.db.Exec("UPDATE teams SET score = score + ? WHERE id = ?", submission.AwardedPoints, teamID)
		}
		go s.InvalidateScoreboardCache()
		go s.InvalidateStatisticsCache()
		go s.notifySolve(userID, target.name, submission.AwardedPoints, target.solveCount+1)
		return c.JSON(http.StatusOK, map[string]any{
			"correct":        true,
			"points":         submission.AwardedPoints,
			"bucket_key":     submission.BucketKey,
			"challenge_id":   target.id,
			"challenge_name": target.name,
		})
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]any{"correct": false})
}

type submissionOutcome struct {
	IsCorrect     bool
	BucketKey     string
	AwardedPoints int
}

func (s *Server) matchStandardOrPentestSubmission(challengeID int64, challengeType, correctFlag, flagType, checkerConfig, submitted string, defaultPoints int) submissionOutcome {
	outcome := submissionOutcome{AwardedPoints: defaultPoints}

	if challengeType == "pentest" {
		cfg := s.loadPentestCheckerConfig(checkerConfig, correctFlag, flagType, defaultPoints)
		for _, spec := range cfg.Flags {
			checkerName := spec.Type
			if checkerName == "" {
				checkerName = "exact"
			}
			checker, _ := plugin.Default.GetFlagChecker(checkerName)
			if checker == nil {
				checker, _ = plugin.Default.GetFlagChecker("exact")
			}
			if checker != nil && checker.Check(spec.Value, submitted) {
				outcome.IsCorrect = true
				outcome.BucketKey = spec.Key
				if outcome.BucketKey == "" {
					outcome.BucketKey = spec.Label
				}
				if outcome.BucketKey == "" {
					outcome.BucketKey = "pentest"
				}
				if spec.Points > 0 {
					outcome.AwardedPoints = spec.Points
				}
				return outcome
			}
		}
		return outcome
	}

	checker, _ := plugin.Default.GetFlagChecker(flagType)
	if checker == nil {
		checker, _ = plugin.Default.GetFlagChecker("exact")
	}
	if checker != nil && checker.Check(correctFlag, submitted) {
		outcome.IsCorrect = true
		return outcome
	}

	flagRows, _ := s.db.Query("SELECT content, type FROM challenge_flags WHERE challenge_id=?", challengeID)
	if flagRows == nil {
		return outcome
	}
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
		if altChecker != nil && altChecker.Check(fc, submitted) {
			outcome.IsCorrect = true
			return outcome
		}
	}
	return outcome
}

func (s *Server) loadPentestCheckerConfig(rawConfig, fallbackFlag, fallbackType string, fallbackPoints int) models.PentestCheckerConfig {
	var cfg models.PentestCheckerConfig
	if strings.TrimSpace(rawConfig) != "" {
		_ = json.Unmarshal([]byte(rawConfig), &cfg)
	}
	if len(cfg.Flags) == 0 && strings.TrimSpace(fallbackFlag) != "" {
		cfg.Flags = []models.ScoredFlagSpec{{
			Key:    "primary",
			Label:  "Primary",
			Value:  fallbackFlag,
			Type:   fallbackType,
			Points: fallbackPoints,
		}}
	}
	for i := range cfg.Flags {
		if cfg.Flags[i].Key == "" {
			cfg.Flags[i].Key = fmt.Sprintf("flag_%d", i+1)
		}
		if cfg.Flags[i].Type == "" {
			cfg.Flags[i].Type = "exact"
		}
		if cfg.Flags[i].Points <= 0 {
			cfg.Flags[i].Points = fallbackPoints
		}
	}
	return cfg
}

func (s *Server) countSolvedByOwner(challengeID, userID, teamID int64) (int, error) {
	var count int
	query := "SELECT COUNT(*) FROM submissions WHERE challenge_id=? AND is_correct=TRUE"
	args := []any{challengeID}
	if s.cfg.CTF.TeamMode && teamID > 0 {
		query += " AND team_id=?"
		args = append(args, teamID)
	} else {
		query += " AND user_id=?"
		args = append(args, userID)
	}
	err := s.db.QueryRow(query, args...).Scan(&count)
	return count, err
}

func (s *Server) bucketAlreadySolved(challengeID int64, bucketKey string, userID, teamID int64) (bool, error) {
	var count int
	query := "SELECT COUNT(*) FROM submissions WHERE challenge_id=? AND bucket_key=? AND is_correct=TRUE"
	args := []any{challengeID, bucketKey}
	if s.cfg.CTF.TeamMode && teamID > 0 {
		query += " AND team_id=?"
		args = append(args, teamID)
	} else {
		query += " AND user_id=?"
		args = append(args, userID)
	}
	err := s.db.QueryRow(query, args...).Scan(&count)
	return count > 0, err
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

func (s *Server) lookupDynamicFlag(challengeID, userID int64, deployType string) (string, bool) {
	deployType = canonicalDeployType(deployType, "dynamic_deploy")
	query := `SELECT dynamic_flag FROM instances WHERE challenge_id=? AND status='running' AND dynamic_flag<>''`
	args := []any{challengeID}
	if deployType == "always_on" {
		query += ` ORDER BY created_at DESC LIMIT 1`
	} else if s.cfg.CTF.TeamMode {
		teamID := s.getTeamIDForUser(userID)
		if teamID == 0 {
			return "", false
		}
		query += ` AND team_id=? ORDER BY created_at DESC LIMIT 1`
		args = append(args, teamID)
	} else {
		query += ` AND user_id=? ORDER BY created_at DESC LIMIT 1`
		args = append(args, userID)
	}
	var flag string
	if err := s.db.QueryRow(query, args...).Scan(&flag); err != nil || flag == "" {
		return "", false
	}
	return flag, true
}

func (s *Server) generateDynamicFlag() string {
	buf := make([]byte, 12)
	if _, err := crand.Read(buf); err != nil {
		return s.cfg.CTF.FlagPrefix + "fallback_dynamic_flag" + s.cfg.CTF.FlagSuffix
	}
	return s.cfg.CTF.FlagPrefix + hex.EncodeToString(buf) + s.cfg.CTF.FlagSuffix
}

// ─── Instance management ──────────────────────────────────────────────────────

func (s *Server) handleGetInstance(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)
	teamID := int64(0)
	if s.cfg.CTF.TeamMode {
		teamID = s.getTeamIDForUser(userID)
	}

	var deployType string
	_ = s.db.QueryRow(`SELECT deploy_type FROM challenges WHERE id=?`, challengeID).Scan(&deployType)
	deployType = canonicalDeployType(deployType, "")
	if inst, ok := s.findRunningInstance(challengeID, userID, teamID, deployType); ok {
		return c.JSON(http.StatusOK, inst)
	}
	var inst models.Instance
	query := `SELECT id, challenge_id, user_id, team_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag, reserved_cpu_mil, reserved_memory_mb, created_at, expires_at, status
		FROM instances WHERE challenge_id=?`
	args := []any{challengeID}
	if deployTypeUsesOwnerBinding(deployType) {
		if s.cfg.CTF.TeamMode {
			query += ` AND team_id=?`
			args = append(args, teamID)
		} else {
			query += ` AND user_id=?`
			args = append(args, userID)
		}
	}
	query += ` ORDER BY created_at DESC LIMIT 1`
	err = s.db.QueryRow(query, args...).Scan(&inst.ID, &inst.ChallengeID, &inst.UserID, &inst.TeamID, &inst.InstanceType, &inst.Backend,
		&inst.TargetID, &inst.InstanceID, &inst.ConnectionInfo, &inst.DynamicFlag, &inst.ReservedCPUMil, &inst.ReservedMemMB, &inst.CreatedAt, &inst.ExpiresAt, &inst.Status)
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
		`SELECT id, challenge_type, deploy_type, deploy_backend, deploy_config, image, vm_template FROM challenges WHERE id=? AND is_visible`,
		challengeID,
	).Scan(&ch.ID, &ch.ChallengeType, &ch.DeployType, &ch.DeployBackend, &ch.DeployConfig, &ch.Image, &ch.VMTemplate)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	ch.DeployType = canonicalDeployType(ch.DeployType, ch.ChallengeType)
	if ch.DeployType == "no_deploy" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "this challenge has no deployable instance"})
	}
	if ch.DeployType == "always_on" && getUserRole(c) != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "this service is provisioned by admins/operators and is not started on demand by players"})
	}

	teamID := int64(0)
	if s.cfg.CTF.TeamMode {
		teamID = s.getTeamIDForUser(userID)
		if deployTypeUsesOwnerBinding(ch.DeployType) && teamID == 0 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "you must be in a team to start this instance"})
		}
	}

	if existing, ok := s.findRunningInstance(challengeID, userID, teamID, ch.DeployType); ok {
		return c.JSON(http.StatusOK, existing)
	}

	var deployConfig map[string]any
	if ch.DeployConfig != "" && ch.DeployConfig != "{}" {
		_ = json.Unmarshal([]byte(ch.DeployConfig), &deployConfig)
	}
	if deployConfig == nil {
		deployConfig = map[string]any{}
	}
	requestedCPU, requestedMemMB := parseRequestedResources(deployConfig)
	selected, back, err := s.resolveDeployerSelection(ch.DeployBackend, requestedCPU, requestedMemMB)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "deployer backend not available"})
	}
	dynamicFlag := ""
	if ch.ChallengeType == "dynamic_deploy" {
		dynamicFlag = s.generateDynamicFlag()
	}
	s.injectRuntimeEnv(deployConfig, &ch, userID, teamID, dynamicFlag)

	req := deployer.DeployRequest{
		ChallengeID:  ch.ID,
		Image:        ch.Image,
		VMTemplate:   ch.VMTemplate,
		DeployConfig: deployConfig,
		DeployType:   ch.DeployType,
		Backend:      selected.Type,
		InstanceTTL:  s.cfg.Deployer.InstanceTTL,
	}
	if deployTypeUsesOwnerBinding(ch.DeployType) {
		req.UserID = &userID
		if teamID > 0 {
			req.TeamID = &teamID
		}
	}

	inst, err := back.Deploy(context.Background(), req)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "deploy failed"})
	}

	// Persist instance
	connInfo := inst.ConnectionInfo
	if connInfo == "" {
		connInfo = "{}"
	}
	if inst.ExpiresAt == nil && deployTypeUsesOwnerBinding(ch.DeployType) {
		if ttl, err := time.ParseDuration(s.cfg.Deployer.InstanceTTL); err == nil && ttl > 0 {
			expiresAt := time.Now().Add(ttl)
			inst.ExpiresAt = &expiresAt
		}
	}
	var dbUserID any
	var dbTeamID any
	if deployTypeUsesOwnerBinding(ch.DeployType) {
		dbUserID = userID
		if teamID > 0 {
			dbTeamID = teamID
		}
	}
	newID, dbErr := s.db.InsertGetID(
		`INSERT INTO instances (challenge_id, user_id, team_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag, reserved_cpu_mil, reserved_memory_mb, expires_at, status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		challengeID, dbUserID, dbTeamID, ch.DeployType, selected.Type, selected.ID, inst.InstanceID, connInfo, dynamicFlag, int(requestedCPU*1000), requestedMemMB, inst.ExpiresAt, "running",
	)
	if dbErr != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error saving instance"})
	}
	inst.ID = newID
	inst.Backend = selected.Type
	inst.TargetID = selected.ID
	inst.ReservedCPUMil = int(requestedCPU * 1000)
	inst.ReservedMemMB = requestedMemMB
	return c.JSON(http.StatusCreated, inst)
}

func (s *Server) findRunningInstance(challengeID, userID, teamID int64, deployType string) (models.Instance, bool) {
	deployType = canonicalDeployType(deployType, "")
	var inst models.Instance
	query := `SELECT id, challenge_id, user_id, team_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag, reserved_cpu_mil, reserved_memory_mb, created_at, expires_at, status
		FROM instances WHERE challenge_id=? AND status='running'`
	args := []any{challengeID}
	if deployTypeUsesOwnerBinding(deployType) {
		if s.cfg.CTF.TeamMode {
			query += ` AND team_id=?`
			args = append(args, teamID)
		} else {
			query += ` AND user_id=?`
			args = append(args, userID)
		}
	}
	query += ` ORDER BY created_at DESC LIMIT 1`
	err := s.db.QueryRow(query, args...).Scan(
		&inst.ID, &inst.ChallengeID, &inst.UserID, &inst.TeamID, &inst.InstanceType, &inst.Backend,
		&inst.TargetID, &inst.InstanceID, &inst.ConnectionInfo, &inst.DynamicFlag, &inst.ReservedCPUMil, &inst.ReservedMemMB, &inst.CreatedAt, &inst.ExpiresAt, &inst.Status,
	)
	return inst, err == nil
}

func (s *Server) handleStopInstance(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)
	teamID := int64(0)
	if s.cfg.CTF.TeamMode {
		teamID = s.getTeamIDForUser(userID)
	}

	var inst models.Instance
	var deployType string
	_ = s.db.QueryRow(`SELECT deploy_type FROM challenges WHERE id=?`, challengeID).Scan(&deployType)
	deployType = canonicalDeployType(deployType, "")
	if deployType == "always_on" && getUserRole(c) != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "shared instances can only be stopped by an admin"})
	}
	query := `SELECT id, backend, target_id, instance_id FROM instances WHERE challenge_id=? AND status='running'`
	args := []any{challengeID}
	if deployTypeUsesOwnerBinding(deployType) {
		if s.cfg.CTF.TeamMode {
			query += ` AND team_id=?`
			args = append(args, teamID)
		} else {
			query += ` AND user_id=?`
			args = append(args, userID)
		}
	}
	query += ` ORDER BY created_at DESC LIMIT 1`
	err = s.db.QueryRow(query, args...).Scan(&inst.ID, &inst.Backend, &inst.TargetID, &inst.InstanceID)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no running instance"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	targetName := inst.TargetID
	if targetName == "" {
		targetName = inst.Backend
	}
	back, err := s.deployer.Get(targetName)
	if err == nil {
		_ = back.Destroy(context.Background(), inst.InstanceID)
	}

	_, _ = s.db.Exec(`UPDATE instances SET status='stopped' WHERE id=?`, inst.ID)
	return c.JSON(http.StatusOK, map[string]string{"message": "instance stopped"})
}

func (s *Server) handleExtendInstance(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	ttl, err := time.ParseDuration(s.cfg.Deployer.InstanceTTL)
	if err != nil || ttl <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "instance ttl is not configured"})
	}

	userID := getUserID(c)
	teamID := int64(0)
	if s.cfg.CTF.TeamMode {
		teamID = s.getTeamIDForUser(userID)
	}

	var deployType string
	_ = s.db.QueryRow(`SELECT deploy_type FROM challenges WHERE id=?`, challengeID).Scan(&deployType)
	deployType = canonicalDeployType(deployType, "")
	if deployType == "always_on" && getUserRole(c) != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "shared instances can only be extended by an admin"})
	}

	inst, ok := s.findRunningInstance(challengeID, userID, teamID, deployType)
	if !ok {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "no running instance"})
	}
	base := time.Now()
	if inst.ExpiresAt != nil && inst.ExpiresAt.After(base) {
		base = *inst.ExpiresAt
	}
	expiresAt := base.Add(ttl)
	if _, err := s.db.Exec(`UPDATE instances SET expires_at=? WHERE id=?`, expiresAt, inst.ID); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error extending instance"})
	}
	inst.ExpiresAt = &expiresAt
	return c.JSON(http.StatusOK, inst)
}

func (s *Server) injectRuntimeEnv(deployConfig map[string]any, ch *models.Challenge, userID, teamID int64, dynamicFlag string) {
	env, _ := deployConfig["env"].(map[string]any)
	if env == nil {
		env = map[string]any{}
	}

	env["HECSTACK_CHALLENGE_ID"] = ch.ID
	env["HECSTACK_CHALLENGE_TYPE"] = ch.ChallengeType
	env["HECSTACK_DEPLOY_TYPE"] = ch.DeployType
	env["HECSTACK_DEPLOY_SELECTOR"] = strings.TrimSpace(ch.DeployBackend)
	if ch.ConnectionInfo != "" {
		env["HECSTACK_CONNECTION_INFO"] = ch.ConnectionInfo
	}
	if userID > 0 {
		env["HECSTACK_USER_ID"] = userID
	}
	if teamID > 0 {
		env["HECSTACK_TEAM_ID"] = teamID
	}
	if dynamicFlag != "" {
		env["FLAG"] = dynamicFlag
		env["DYNAMIC_FLAG"] = dynamicFlag
	}
	if (ch.ChallengeType == "pentest" || ch.ChallengeType == "attack_defence_defense") && s.cfg.AD.VPN.Enabled {
		if s.cfg.AD.VPN.ServerEndpoint != "" {
			env["HECSTACK_VPN_ENDPOINT"] = s.cfg.AD.VPN.ServerEndpoint
		}
		if s.cfg.AD.VPN.ServerPublicKey != "" {
			env["HECSTACK_VPN_SERVER_PUBLIC_KEY"] = s.cfg.AD.VPN.ServerPublicKey
		}
		if s.cfg.AD.VPN.ServerIP != "" {
			env["HECSTACK_VPN_SERVER_IP"] = s.cfg.AD.VPN.ServerIP
		}
		if s.cfg.AD.VPN.GameNetCIDR != "" {
			env["HECSTACK_VPN_GAME_NET_CIDR"] = s.cfg.AD.VPN.GameNetCIDR
		}
		if s.cfg.AD.VPN.DNS != "" {
			env["HECSTACK_VPN_DNS"] = s.cfg.AD.VPN.DNS
		}
	}
	if ch.ChallengeType == "attack_defence_attack" {
		env["HECSTACK_ATTACK_MODE"] = "true"
	}
	if ch.ChallengeType == "attack_defence_defense" {
		env["HECSTACK_DEFENSE_MODE"] = "true"
	}
	deployConfig["env"] = env
}

func parseRequestedResources(deployConfig map[string]any) (float64, int) {
	if deployConfig == nil {
		return 0, 0
	}
	if resources, ok := deployConfig["resources"].(map[string]any); ok {
		cpu := parseCPUValue(resources["cpu"])
		if cpu == 0 {
			cpu = parseCPUValue(resources["cpus"])
		}
		mem := parseMemoryMB(resources["memory_mb"])
		if mem == 0 {
			mem = parseMemoryMB(resources["memory"])
		}
		if cpu > 0 || mem > 0 {
			return cpu, mem
		}
	}
	cpu := parseCPUValue(deployConfig["cpus"])
	if cpu == 0 {
		cpu = parseCPUValue(deployConfig["cpu"])
	}
	mem := parseMemoryMB(deployConfig["memory_mb"])
	if mem == 0 {
		mem = parseMemoryMB(deployConfig["memory"])
	}
	return cpu, mem
}

func parseCPUValue(value any) float64 {
	switch v := value.(type) {
	case float64:
		if v > 0 {
			return v
		}
	case float32:
		if v > 0 {
			return float64(v)
		}
	case int:
		if v > 0 {
			return float64(v)
		}
	case int64:
		if v > 0 {
			return float64(v)
		}
	case json.Number:
		f, _ := v.Float64()
		if f > 0 {
			return f
		}
	case string:
		f, _ := strconv.ParseFloat(strings.TrimSpace(v), 64)
		if f > 0 {
			return f
		}
	}
	return 0
}

func parseMemoryMB(value any) int {
	switch v := value.(type) {
	case int:
		if v > 0 {
			return v
		}
	case int64:
		if v > 0 {
			return int(v)
		}
	case float64:
		if v > 0 {
			return int(v)
		}
	case json.Number:
		i, _ := v.Int64()
		if i > 0 {
			return int(i)
		}
	case string:
		s := strings.TrimSpace(strings.ToLower(v))
		if s == "" {
			return 0
		}
		multiplier := 1
		switch {
		case strings.HasSuffix(s, "g"), strings.HasSuffix(s, "gb"), strings.HasSuffix(s, "gi"), strings.HasSuffix(s, "gib"):
			multiplier = 1024
			s = strings.TrimRight(s, "bgik")
		case strings.HasSuffix(s, "m"), strings.HasSuffix(s, "mb"), strings.HasSuffix(s, "mi"), strings.HasSuffix(s, "mib"):
			s = strings.TrimRight(s, "bmik")
		}
		f, _ := strconv.ParseFloat(strings.TrimSpace(s), 64)
		if f > 0 {
			return int(f * float64(multiplier))
		}
	}
	return 0
}

func (s *Server) currentDeployTargetUsage() map[string]deployer.TargetUsage {
	rows, err := s.db.Query(`SELECT target_id, COUNT(*), COALESCE(SUM(reserved_cpu_mil), 0), COALESCE(SUM(reserved_memory_mb), 0)
		FROM instances
		WHERE status='running' AND target_id<>''
		GROUP BY target_id`)
	if err != nil {
		return map[string]deployer.TargetUsage{}
	}
	defer rows.Close()

	usage := map[string]deployer.TargetUsage{}
	for rows.Next() {
		var targetID string
		var instances int
		var cpuMil int
		var memoryMB int
		if err := rows.Scan(&targetID, &instances, &cpuMil, &memoryMB); err != nil {
			continue
		}
		usage[targetID] = deployer.TargetUsage{
			Instances: instances,
			CPU:       float64(cpuMil) / 1000.0,
			MemoryMB:  memoryMB,
		}
	}
	return usage
}

func (s *Server) resolveDeployerSelection(selector string, cpu float64, memoryMB int) (deployer.Selection, deployer.Deployer, error) {
	trimmed := strings.TrimSpace(selector)
	if trimmed == "" {
		trimmed = "auto"
	}

	usage := s.currentDeployTargetUsage()
	if selection, err := s.deployer.Select(trimmed, usage, cpu, memoryMB); err == nil {
		return selection, selection.Deployer, nil
	}

	if trimmed == "auto" {
		for _, fallback := range []string{"docker", "kubernetes", "pve"} {
			back, err := s.deployer.Get(fallback)
			if err == nil {
				return deployer.Selection{ID: fallback, Type: back.Type(), Deployer: back}, back, nil
			}
		}
		return deployer.Selection{}, nil, fmt.Errorf("no deploy targets or backends available")
	}

	back, err := s.deployer.Get(trimmed)
	if err != nil {
		return deployer.Selection{}, nil, err
	}
	return deployer.Selection{ID: trimmed, Type: back.Type(), Deployer: back}, back, nil
}

func (s *Server) handleAdminListChallenges(c echo.Context) error {
	rows, err := s.db.Query(`SELECT id, name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, image, vm_template, is_visible, connection_info, max_attempts, created_at FROM challenges ORDER BY id`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var challenges []models.Challenge
	for rows.Next() {
		var ch models.Challenge
		if err := rows.Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points, &ch.Flag, &ch.ChallengeType, &ch.FlagType, &ch.CheckerConfig, &ch.DeployType, &ch.DeployBackend, &ch.DeployConfig, &ch.Image, &ch.VMTemplate, &ch.IsVisible, &ch.ConnectionInfo, &ch.MaxAttempts, &ch.CreatedAt); err != nil {
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
	s.normalizeChallenge(&ch)
	id, err := s.db.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, image, vm_template, is_visible, connection_info, max_attempts) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		ch.Name, ch.Description, ch.Category, ch.Points, ch.Flag, ch.ChallengeType, ch.FlagType, ch.CheckerConfig, ch.DeployType, ch.DeployBackend, ch.DeployConfig, ch.Image, ch.VMTemplate, ch.IsVisible, ch.ConnectionInfo, ch.MaxAttempts,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	ch.ID = id
	s.InvalidateStatisticsCache()
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
	s.normalizeChallenge(&ch)
	_, err = s.db.Exec(
		`UPDATE challenges SET name=?, description=?, category=?, points=?, flag=?, challenge_type=?, flag_type=?, checker_config=?, deploy_type=?, deploy_backend=?, deploy_config=?, image=?, vm_template=?, is_visible=?, connection_info=?, max_attempts=? WHERE id=?`,
		ch.Name, ch.Description, ch.Category, ch.Points, ch.Flag, ch.ChallengeType, ch.FlagType, ch.CheckerConfig, ch.DeployType, ch.DeployBackend, ch.DeployConfig, ch.Image, ch.VMTemplate, ch.IsVisible, ch.ConnectionInfo, ch.MaxAttempts, id,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	ch.ID = id
	s.InvalidateStatisticsCache()
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
	s.InvalidateStatisticsCache()
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
	return c.JSON(http.StatusOK, plugin.Default.ChallengeTypes())
}

func (s *Server) normalizeChallenge(ch *models.Challenge) {
	if ch.ChallengeType == "" {
		ch.ChallengeType = "static"
	}
	if ch.FlagType == "" {
		ch.FlagType = "exact"
	}
	ch.DeployType = canonicalDeployType(ch.DeployType, ch.ChallengeType)
	if ch.DeployConfig == "" {
		ch.DeployConfig = "{}"
	}
	if ch.CheckerConfig == "" {
		ch.CheckerConfig = "{}"
	}

	if ch.DeployType != "no_deploy" && strings.TrimSpace(ch.DeployBackend) == "" {
		ch.DeployBackend = "auto"
	}
}

func canonicalDeployType(raw, challengeType string) string {
	value := strings.TrimSpace(strings.ToLower(raw))
	switch value {
	case "", "none":
		value = "no_deploy"
	case "single_instance", "attack_defence", "shared", "shared_service":
		value = "always_on"
	case "per_user", "per_team", "instance", "per_instance_deploy":
		value = "per_instance"
	}

	switch strings.TrimSpace(strings.ToLower(challengeType)) {
	case "dynamic_deploy":
		if value == "no_deploy" {
			return "per_instance"
		}
	case "pentest", "attack_defence_attack", "attack_defence_defense":
		if value == "no_deploy" {
			return "always_on"
		}
	}
	if value == "" {
		return "no_deploy"
	}
	return value
}

func deployTypeUsesOwnerBinding(deployType string) bool {
	return canonicalDeployType(deployType, "") == "per_instance"
}
