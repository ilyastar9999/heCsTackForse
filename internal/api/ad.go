package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/ad"
	"github.com/ilyastar9999/heCsTackForse/internal/cache"
	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

const maxSploitScriptSize = 1 << 20 // 1 MiB

// ────────────────────────────────────────────────────────────────────────────
// AD status
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADStatus(c echo.Context) error {
	round := int64(0)
	if s.adEngine != nil {
		round = s.adEngine.CurrentRound()
	}
	return c.JSON(http.StatusOK, map[string]any{
		"round":           round,
		"round_duration":  s.cfg.AD.RoundDuration,
		"flag_submit_url": s.cfg.AD.FlagSubmitURL,
		"vpn_enabled":     s.cfg.AD.VPN.Enabled,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// AD scoreboard — attack/defence points per team
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADScoreboard(c echo.Context) error {
	ttl := cache.ParseDuration(s.cfg.Cache.TTLAD, 5*time.Second)
	mode := "team"
	if !s.cfg.CTF.TeamMode {
		mode = "user"
	}
	cacheKey := cache.Key("ad", "scoreboard", mode)

	type adEntry struct {
		TeamID     int64  `json:"team_id"`
		TeamName   string `json:"team_name"`
		DefencePts int64  `json:"defence_pts"`
		AttackPts  int64  `json:"attack_pts"`
		Total      int64  `json:"total"`
	}

	var board []adEntry
	if s.cache.Get(context.Background(), cacheKey, &board) {
		return c.JSON(http.StatusOK, board)
	}

	query := `
		SELECT t.id, t.name,
		       COALESCE(defs.defence_pts, 0)                                  AS defence_pts,
		       COALESCE(atk.attack_pts, 0)                                    AS attack_pts
		FROM teams t
		LEFT JOIN (
			SELECT team_id, SUM(score) AS defence_pts
			FROM ad_services
			GROUP BY team_id
		) defs ON defs.team_id = t.id
		LEFT JOIN (
			SELECT sp.team_id, SUM(sr.awarded_points) AS attack_pts
			FROM ad_sploit_results sr
			JOIN ad_sploits sp ON sp.id = sr.sploit_id
			WHERE sr.flags_submitted > 0 OR sr.bucket_key = 'all_teams_bonus'
			GROUP BY sp.team_id
		) atk ON atk.team_id = t.id
		ORDER BY (COALESCE(defs.defence_pts, 0) + COALESCE(atk.attack_pts, 0)) DESC
	`
	if !s.cfg.CTF.TeamMode {
		query = `
			SELECT u.id, u.username,
			       COALESCE(defs.defence_pts, 0)                                  AS defence_pts,
			       COALESCE(atk.attack_pts, 0)                                    AS attack_pts
			FROM users u
			LEFT JOIN (
				SELECT team_id, SUM(score) AS defence_pts
				FROM ad_services
				GROUP BY team_id
			) defs ON defs.team_id = u.id
			LEFT JOIN (
				SELECT sp.team_id, SUM(sr.awarded_points) AS attack_pts
				FROM ad_sploit_results sr
				JOIN ad_sploits sp ON sp.id = sr.sploit_id
				WHERE sr.flags_submitted > 0 OR sr.bucket_key = 'all_teams_bonus'
				GROUP BY sp.team_id
			) atk ON atk.team_id = u.id
			ORDER BY (COALESCE(defs.defence_pts, 0) + COALESCE(atk.attack_pts, 0)) DESC
		`
	}
	rows, err := s.db.Query(query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	for rows.Next() {
		var e adEntry
		if err := rows.Scan(&e.TeamID, &e.TeamName, &e.DefencePts, &e.AttackPts); err != nil {
			continue
		}
		e.Total = e.DefencePts + e.AttackPts
		board = append(board, e)
	}
	if board == nil {
		board = []adEntry{}
	}
	s.cache.Set(context.Background(), cacheKey, board, ttl)
	return c.JSON(http.StatusOK, board)
}

// ────────────────────────────────────────────────────────────────────────────
// Service status grid
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADServices(c echo.Context) error {
	rows, err := s.db.Query(`
		SELECT svc.challenge_id, ch.name, ch.checker_config, svc.team_id, svc.status, svc.round, svc.score, svc.checked_at
		FROM ad_services svc
		JOIN challenges ch ON ch.id = svc.challenge_id
		INNER JOIN (
		    SELECT challenge_id, team_id, MAX(round) AS max_round
		    FROM ad_services
		    GROUP BY challenge_id, team_id
		) latest ON svc.challenge_id=latest.challenge_id
		        AND svc.team_id=latest.team_id
		        AND svc.round=latest.max_round
		ORDER BY svc.challenge_id, svc.team_id
	`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var statuses []models.ADServiceStatus
	for rows.Next() {
		var st models.ADServiceStatus
		var checkerConfig string
		if err := rows.Scan(&st.ChallengeID, &st.ChallengeName, &checkerConfig, &st.TeamID, &st.Status, &st.Round, &st.Score, &st.CheckedAt); err != nil {
			continue
		}
		st.MaxScore = adServiceMaxScore(checkerConfig)
		statuses = append(statuses, st)
	}
	if statuses == nil {
		statuses = []models.ADServiceStatus{}
	}
	return c.JSON(http.StatusOK, statuses)
}

func adServiceMaxScore(checkerConfig string) int {
	if strings.TrimSpace(checkerConfig) == "" {
		return 0
	}
	var cfg models.ADCheckerConfig
	if err := json.Unmarshal([]byte(checkerConfig), &cfg); err != nil {
		return 0
	}
	total := 0
	for _, check := range cfg.DefenseChecks {
		if check.Points > 0 {
			total += check.Points
		}
	}
	return total
}

func (s *Server) handleADCatalog(c echo.Context) error {
	rows, err := s.db.Query(`
		SELECT id, name, checker_config
		FROM challenges
		WHERE is_visible=TRUE AND challenge_type='attack_defence_attack'
		ORDER BY category, points, id
	`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	type bucketEntry struct {
		Key    string `json:"key"`
		Label  string `json:"label"`
		Points int    `json:"points"`
	}
	type challengeEntry struct {
		ID      int64         `json:"id"`
		Name    string        `json:"name"`
		Buckets []bucketEntry `json:"buckets"`
	}

	var catalog []challengeEntry
	for rows.Next() {
		var id int64
		var name string
		var checkerConfig string
		if err := rows.Scan(&id, &name, &checkerConfig); err != nil {
			continue
		}
		entry := challengeEntry{ID: id, Name: name, Buckets: []bucketEntry{}}
		var cfg models.ADCheckerConfig
		if strings.TrimSpace(checkerConfig) != "" && json.Unmarshal([]byte(checkerConfig), &cfg) == nil {
			for _, bucket := range cfg.Buckets() {
				label := strings.TrimSpace(bucket.Label)
				if label == "" {
					label = strings.TrimSpace(bucket.Name)
				}
				if label == "" {
					label = bucket.Key
				}
				entry.Buckets = append(entry.Buckets, bucketEntry{
					Key:    bucket.Key,
					Label:  label,
					Points: bucket.Points,
				})
			}
		}
		catalog = append(catalog, entry)
	}
	if catalog == nil {
		catalog = []challengeEntry{}
	}
	return c.JSON(http.StatusOK, catalog)
}

// ────────────────────────────────────────────────────────────────────────────
// VPN config download
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADGetVPN(c echo.Context) error {
	if !s.cfg.AD.VPN.Enabled {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "VPN not enabled"})
	}
	ownerID := s.getTeamIDForUser(getUserID(c))
	if ownerID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "VPN owner is not available for this account"})
	}

	peer, teamIP, err := s.ensureADVPNPeer(ownerID, false)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "internal error"})
	}
	cfg := s.cfg.AD.VPN

	conf := ad.BuildClientConfig(
		peer.PrivateKey,
		teamIP,
		cfg.ServerPublicKey,
		cfg.ServerEndpoint,
		cfg.ServerIP,
		cfg.GameNetCIDR,
		cfg.DNS,
	)

	c.Response().Header().Set("Content-Type", "text/plain; charset=utf-8")
	ownerPrefix := "user"
	if s.cfg.CTF.TeamMode {
		ownerPrefix = "team"
	}
	c.Response().Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s%d-wg.conf"`, ownerPrefix, ownerID))
	return c.String(http.StatusOK, conf)
}

func (s *Server) handleADVPNStatus(c echo.Context) error {
	if !s.cfg.AD.VPN.Enabled {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "VPN not enabled"})
	}
	ownerID := s.getTeamIDForUser(getUserID(c))
	if ownerID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "VPN owner is not available for this account"})
	}

	peer, teamIP, err := s.ensureADVPNPeer(ownerID, false)
	if err != nil && peer.ID == 0 {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "internal error"})
	}

	ownerType := "user"
	if s.cfg.CTF.TeamMode {
		ownerType = "team"
	}
	cfg := s.cfg.AD.VPN
	payload := map[string]any{
		"enabled":         true,
		"owner_type":      ownerType,
		"owner_id":        ownerID,
		"provisioned":     peer.Provisioned,
		"last_sync_error": peer.LastSyncError,
		"client_address":  teamIP,
		"allowed_subnet":  peer.AllowedIP,
		"server_endpoint": cfg.ServerEndpoint,
		"server_ip":       cfg.ServerIP,
		"game_net_cidr":   cfg.GameNetCIDR,
		"dns":             cfg.DNS,
		"download_url":    "/api/ad/vpn",
		"sync_url":        "/api/ad/vpn/sync",
	}
	if err != nil {
		payload["last_sync_error"] = "sync failed"
	}
	return c.JSON(http.StatusOK, payload)
}

func (s *Server) handleADSyncVPN(c echo.Context) error {
	if !s.cfg.AD.VPN.Enabled {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "VPN not enabled"})
	}
	ownerID := s.getTeamIDForUser(getUserID(c))
	if ownerID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "VPN owner is not available for this account"})
	}

	peer, teamIP, err := s.ensureADVPNPeer(ownerID, true)
	if err != nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]any{
			"error":       "internal error",
			"provisioned": peer.Provisioned,
			"last_error":  peer.LastSyncError,
		})
	}

	return c.JSON(http.StatusOK, map[string]any{
		"message":        "VPN access synchronized",
		"provisioned":    peer.Provisioned,
		"last_error":     peer.LastSyncError,
		"client_address": teamIP,
		"allowed_subnet": peer.AllowedIP,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Sploit CRUD
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADListSploits(c echo.Context) error {
	teamID := s.getTeamIDForUser(getUserID(c))
	if teamID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "you must be in a team"})
	}
	rows, err := s.db.Query(
		`SELECT id, team_id, challenge_id, bucket_key, name, language, enabled, created_at, last_run_at
		 FROM ad_sploits WHERE team_id=? ORDER BY id`,
		teamID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var sploits []models.Sploit
	for rows.Next() {
		var sp models.Sploit
		if err := rows.Scan(&sp.ID, &sp.TeamID, &sp.ChallengeID, &sp.BucketKey, &sp.Name,
			&sp.Language, &sp.Enabled, &sp.CreatedAt, &sp.LastRunAt); err != nil {
			continue
		}
		sploits = append(sploits, sp)
	}
	if sploits == nil {
		sploits = []models.Sploit{}
	}
	return c.JSON(http.StatusOK, sploits)
}

func (s *Server) handleADCreateSploit(c echo.Context) error {
	teamID := s.getTeamIDForUser(getUserID(c))
	if teamID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "you must be in a team"})
	}
	var req struct {
		ChallengeID int64  `json:"challenge_id"`
		BucketKey   string `json:"bucket_key"`
		Name        string `json:"name"`
		Language    string `json:"language"`
		Script      string `json:"script"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Script == "" || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name and script are required"})
	}
	if len(req.Script) > maxSploitScriptSize {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "script exceeds maximum size"})
	}
	if req.ChallengeID <= 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "a specific attack challenge must be selected"})
	}
	if req.Language == "" {
		req.Language = "python3"
	}
	if err := s.validateADSploitTarget(req.ChallengeID, req.BucketKey); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	id, err := s.db.InsertGetID(
		`INSERT INTO ad_sploits (team_id, challenge_id, bucket_key, name, language, script, enabled) VALUES (?,?,?,?,?,?,TRUE)`,
		teamID, req.ChallengeID, req.BucketKey, req.Name, req.Language, req.Script,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusCreated, map[string]any{"id": id, "message": "sploit created"})
}

func (s *Server) handleADUpdateSploit(c echo.Context) error {
	sploitID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	teamID := s.getTeamIDForUser(getUserID(c))

	var req struct {
		ChallengeID *int64  `json:"challenge_id"`
		BucketKey   *string `json:"bucket_key"`
		Name        string  `json:"name"`
		Language    string  `json:"language"`
		Script      string  `json:"script"`
		Enabled     *bool   `json:"enabled"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}

	// Verify ownership
	var ownerTeam int64
	if err := s.db.QueryRow(`SELECT team_id FROM ad_sploits WHERE id=?`, sploitID).Scan(&ownerTeam); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if ownerTeam != teamID && getUserRole(c) != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}

	if req.Enabled != nil {
		enabled := 0
		if *req.Enabled {
			enabled = 1
		}
		_, _ = s.db.Exec(`UPDATE ad_sploits SET enabled=? WHERE id=?`, enabled, sploitID)
	}
	if req.Script != "" {
		_, _ = s.db.Exec(`UPDATE ad_sploits SET script=? WHERE id=?`, req.Script, sploitID)
	}
	if req.ChallengeID != nil || req.BucketKey != nil {
		var challengeID int64
		var bucketKey string
		_ = s.db.QueryRow(`SELECT challenge_id, bucket_key FROM ad_sploits WHERE id=?`, sploitID).Scan(&challengeID, &bucketKey)
		if req.ChallengeID != nil {
			challengeID = *req.ChallengeID
		}
		if req.BucketKey != nil {
			bucketKey = strings.TrimSpace(*req.BucketKey)
		}
		if err := s.validateADSploitTarget(challengeID, bucketKey); err != nil {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
		}
		_, _ = s.db.Exec(`UPDATE ad_sploits SET challenge_id=?, bucket_key=? WHERE id=?`, challengeID, bucketKey, sploitID)
	}
	if req.Name != "" {
		_, _ = s.db.Exec(`UPDATE ad_sploits SET name=? WHERE id=?`, req.Name, sploitID)
	}
	if req.Language != "" {
		_, _ = s.db.Exec(`UPDATE ad_sploits SET language=? WHERE id=?`, req.Language, sploitID)
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "updated"})
}

func (s *Server) handleADDeleteSploit(c echo.Context) error {
	sploitID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	teamID := s.getTeamIDForUser(getUserID(c))
	var ownerTeam int64
	if err := s.db.QueryRow(`SELECT team_id FROM ad_sploits WHERE id=?`, sploitID).Scan(&ownerTeam); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if ownerTeam != teamID && getUserRole(c) != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}
	_, _ = s.db.Exec(`DELETE FROM ad_sploit_results WHERE sploit_id=?`, sploitID)
	_, _ = s.db.Exec(`DELETE FROM ad_sploits WHERE id=?`, sploitID)
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

func (s *Server) handleADSploitResults(c echo.Context) error {
	sploitID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	teamID := s.getTeamIDForUser(getUserID(c))
	var ownerTeam int64
	if err := s.db.QueryRow(`SELECT team_id FROM ad_sploits WHERE id=?`, sploitID).Scan(&ownerTeam); err != nil {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if ownerTeam != teamID && getUserRole(c) != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "forbidden"})
	}
	rows, err := s.db.Query(`
		SELECT id, sploit_id, target_team_id, round, bucket_key, awarded_points, stdout, flags_captured, flags_submitted, error, ran_at
		FROM ad_sploit_results WHERE sploit_id=? ORDER BY round DESC LIMIT 200`,
		sploitID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var results []models.SploitResult
	for rows.Next() {
		var res models.SploitResult
		if err := rows.Scan(&res.ID, &res.SploitID, &res.TargetTeamID, &res.Round, &res.BucketKey, &res.AwardedPoints, &res.Stdout,
			&res.FlagsCaptured, &res.FlagsSubmitted, &res.Error, &res.RanAt); err != nil {
			continue
		}
		results = append(results, res)
	}
	if results == nil {
		results = []models.SploitResult{}
	}
	return c.JSON(http.StatusOK, results)
}

// ────────────────────────────────────────────────────────────────────────────
// Manual flag submission proxy (player submits a captured flag directly)
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADSubmitFlag(c echo.Context) error {
	var req struct {
		Flags []string `json:"flags"`
	}
	if err := c.Bind(&req); err != nil || len(req.Flags) == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "provide flags array"})
	}
	if len(req.Flags) > 100 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "too many flags, maximum 100 per request"})
	}
	for _, f := range req.Flags {
		if len(f) > 1024 {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "flag too long"})
		}
	}
	if s.cfg.AD.FlagSubmitURL == "" {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "central flag submitter not configured"})
	}
	// Store as ad_flags for record-keeping
	userID := getUserID(c)
	teamID := s.getTeamIDForUser(userID)
	round := int64(0)
	if s.adEngine != nil {
		round = s.adEngine.CurrentRound()
	}
	for _, flag := range req.Flags {
		_, _ = s.db.Exec(
			`INSERT INTO ad_flags (challenge_id, team_id, flag, round) VALUES (0, ?, ?, ?)`,
			teamID, flag, round,
		)
	}

	// Proxy to central submitter
	body, _ := json.Marshal(req.Flags)
	httpReq, err := http.NewRequest(http.MethodPost, s.cfg.AD.FlagSubmitURL, jsonBody(body))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to build request"})
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.cfg.AD.FlagSubmitKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.cfg.AD.FlagSubmitKey)
		httpReq.Header.Set("X-Team-Token", s.cfg.AD.FlagSubmitKey)
	}
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": "flag submitter unreachable"})
	}
	defer resp.Body.Close()
	return c.JSON(http.StatusOK, map[string]any{
		"submitted":     len(req.Flags),
		"server_status": resp.StatusCode,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

// getTeamIDForUser looks up which team the user belongs to.
func (s *Server) getTeamIDForUser(userID int64) int64 {
	if !s.cfg.CTF.TeamMode {
		return userID
	}
	var teamID int64
	_ = s.db.QueryRow(
		`SELECT team_id FROM team_members WHERE user_id=? LIMIT 1`, userID,
	).Scan(&teamID)
	return teamID
}

func (s *Server) validateADSploitTarget(challengeID int64, bucketKey string) error {
	bucketKey = strings.TrimSpace(bucketKey)
	if challengeID == 0 {
		if bucketKey != "" {
			return fmt.Errorf("bucket_key requires a specific attack-defence challenge")
		}
		return nil
	}

	var challengeType string
	var checkerConfig string
	if err := s.db.QueryRow(`SELECT challenge_type, checker_config FROM challenges WHERE id=?`, challengeID).Scan(&challengeType, &checkerConfig); err != nil {
		if err == sql.ErrNoRows {
			return fmt.Errorf("challenge not found")
		}
		return fmt.Errorf("failed to load challenge")
	}
	if challengeType != "attack_defence_attack" {
		return fmt.Errorf("challenge %d is not an attack-defence attack challenge", challengeID)
	}
	if bucketKey == "" {
		return nil
	}

	var cfg models.ADCheckerConfig
	if strings.TrimSpace(checkerConfig) == "" {
		return fmt.Errorf("bucket %q is not configured for this challenge", bucketKey)
	}
	if err := json.Unmarshal([]byte(checkerConfig), &cfg); err != nil {
		return fmt.Errorf("invalid challenge checker_config")
	}
	for _, bucket := range cfg.Buckets() {
		if bucket.Key == bucketKey {
			return nil
		}
	}
	return fmt.Errorf("bucket %q is not configured for this challenge", bucketKey)
}

func (s *Server) handleADListChallengeSploits(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	teamID := s.getTeamIDForUser(getUserID(c))
	if teamID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "you must be in a team"})
	}
	rows, err := s.db.Query(
		`SELECT id, team_id, challenge_id, bucket_key, name, language, enabled, created_at, last_run_at
		 FROM ad_sploits WHERE team_id=? AND challenge_id=? ORDER BY id`,
		teamID, challengeID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var sploits []models.Sploit
	for rows.Next() {
		var sp models.Sploit
		if err := rows.Scan(&sp.ID, &sp.TeamID, &sp.ChallengeID, &sp.BucketKey, &sp.Name,
			&sp.Language, &sp.Enabled, &sp.CreatedAt, &sp.LastRunAt); err != nil {
			continue
		}
		sploits = append(sploits, sp)
	}
	if sploits == nil {
		sploits = []models.Sploit{}
	}
	return c.JSON(http.StatusOK, sploits)
}

func (s *Server) handleADCreateChallengeSploit(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	teamID := s.getTeamIDForUser(getUserID(c))
	if teamID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "you must be in a team"})
	}
	var req struct {
		BucketKey string `json:"bucket_key"`
		Name      string `json:"name"`
		Language  string `json:"language"`
		Script    string `json:"script"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Script == "" || req.Name == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "name and script are required"})
	}
	if len(req.Script) > maxSploitScriptSize {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "script exceeds maximum size"})
	}
	if req.Language == "" {
		req.Language = "python3"
	}
	if err := s.validateADSploitTarget(challengeID, req.BucketKey); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	id, err := s.db.InsertGetID(
		`INSERT INTO ad_sploits (team_id, challenge_id, bucket_key, name, language, script, enabled) VALUES (?,?,?,?,?,?,TRUE)`,
		teamID, challengeID, req.BucketKey, req.Name, req.Language, req.Script,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusCreated, map[string]any{"id": id, "message": "sploit created"})
}

func (s *Server) ensureADVPNPeer(ownerID int64, forceSync bool) (models.VPNPeer, string, error) {
	if ownerID == 0 {
		return models.VPNPeer{}, "", fmt.Errorf("VPN owner is not available for this account")
	}
	if !s.cfg.AD.VPN.Enabled {
		return models.VPNPeer{}, "", fmt.Errorf("VPN not enabled")
	}
	if strings.TrimSpace(s.cfg.AD.VPN.ServerPublicKey) == "" || strings.TrimSpace(s.cfg.AD.VPN.ServerEndpoint) == "" || strings.TrimSpace(s.cfg.AD.VPN.GameNetCIDR) == "" {
		return models.VPNPeer{}, "", fmt.Errorf("VPN is enabled but not fully configured")
	}

	base := s.cfg.AD.VPN.TeamSubnetBase
	if base == "" {
		base = "10.8."
	}
	allowedIP := fmt.Sprintf("%s%d.0/24", base, ownerID)
	teamIP := fmt.Sprintf("%s%d.1/24", base, ownerID)

	var peer models.VPNPeer
	err := s.db.QueryRow(
		`SELECT id, team_id, private_key, public_key, allowed_ip, provisioned, last_sync_error, synced_at, created_at FROM ad_vpn_peers WHERE team_id=?`,
		ownerID,
	).Scan(&peer.ID, &peer.TeamID, &peer.PrivateKey, &peer.PublicKey, &peer.AllowedIP, &peer.Provisioned, &peer.LastSyncError, &peer.SyncedAt, &peer.CreatedAt)
	if err == sql.ErrNoRows {
		privB64, pubB64, genErr := ad.GenerateWireGuardKeys()
		if genErr != nil {
			return models.VPNPeer{}, "", fmt.Errorf("failed to generate VPN keys: %w", genErr)
		}
		newID, insErr := s.db.InsertGetID(
			`INSERT INTO ad_vpn_peers (team_id, private_key, public_key, allowed_ip, provisioned, last_sync_error) VALUES (?, ?, ?, ?, ?, ?)`,
			ownerID, privB64, pubB64, allowedIP, false, "",
		)
		if insErr != nil {
			return models.VPNPeer{}, "", fmt.Errorf("db error: %w", insErr)
		}
		peer = models.VPNPeer{
			ID:            newID,
			TeamID:        ownerID,
			PrivateKey:    privB64,
			PublicKey:     pubB64,
			AllowedIP:     allowedIP,
			Provisioned:   false,
			LastSyncError: "",
		}
	} else if err != nil {
		return models.VPNPeer{}, "", fmt.Errorf("db error")
	}

	if peer.AllowedIP != allowedIP {
		if _, err := s.db.Exec(`UPDATE ad_vpn_peers SET allowed_ip=?, provisioned=FALSE, last_sync_error='', synced_at=NULL WHERE id=?`, allowedIP, peer.ID); err != nil {
			return models.VPNPeer{}, "", fmt.Errorf("db error: %w", err)
		}
		peer.AllowedIP = allowedIP
		peer.Provisioned = false
		peer.LastSyncError = ""
		peer.SyncedAt = nil
	}

	if !peer.Provisioned || forceSync {
		if err := ad.RunVPNHook(s.cfg.AD.VPN, "provision", peer, teamIP); err != nil {
			peer.Provisioned = false
			peer.LastSyncError = "sync failed"
			peer.SyncedAt = nil
			_, _ = s.db.Exec(`UPDATE ad_vpn_peers SET provisioned=FALSE, last_sync_error=?, synced_at=NULL WHERE id=?`, peer.LastSyncError, peer.ID)
			return peer, teamIP, err
		}
		peer.Provisioned = true
		peer.LastSyncError = ""
		_, _ = s.db.Exec(`UPDATE ad_vpn_peers SET provisioned=TRUE, last_sync_error='', synced_at=CURRENT_TIMESTAMP WHERE id=?`, peer.ID)
		if scanErr := s.db.QueryRow(`SELECT synced_at FROM ad_vpn_peers WHERE id=?`, peer.ID).Scan(&peer.SyncedAt); scanErr != nil {
			peer.SyncedAt = nil
		}
	}

	return peer, teamIP, nil
}

// InvalidateADCache removes cached AD scoreboard and services data.
func (s *Server) InvalidateADCache() {
	if s.cache != nil {
		s.cache.InvalidatePrefix(context.Background(), "ad:*")
	}
}
