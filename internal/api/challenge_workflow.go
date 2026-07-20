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

	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type challengeWorkflowResponse struct {
	ChallengeID     int64                    `json:"challenge_id"`
	ChallengeType   string                   `json:"challenge_type"`
	OwnerID         int64                    `json:"owner_id"`
	TeamMode        bool                     `json:"team_mode"`
	CurrentRound    int64                    `json:"current_round"`
	CanAdminRestart bool                     `json:"can_admin_restart,omitempty"`
	VPNEnabled      bool                     `json:"vpn_enabled"`
	VPNDownload     string                   `json:"vpn_download_url,omitempty"`
	VPNSyncURL      string                   `json:"vpn_sync_url,omitempty"`
	VPN             *challengeWorkflowVPN    `json:"vpn,omitempty"`
	RestartVote     *challengeRestartVote    `json:"restart_vote,omitempty"`
	ServiceInstance *challengeServiceRuntime `json:"service_instance,omitempty"`
	RestartEvents   []challengeRestartEvent  `json:"restart_events,omitempty"`
	Languages       []string                 `json:"languages,omitempty"`
	Sploits         []models.Sploit          `json:"sploits,omitempty"`
	ServiceStatus   *models.ADServiceStatus  `json:"service_status,omitempty"`
	ServiceRounds   []models.ADServiceStatus `json:"service_rounds,omitempty"`
	Notes           []string                 `json:"notes,omitempty"`
}

type challengeWorkflowVPN struct {
	OwnerType      string `json:"owner_type"`
	OwnerID        int64  `json:"owner_id"`
	ClientAddress  string `json:"client_address"`
	AllowedSubnet  string `json:"allowed_subnet"`
	ServerEndpoint string `json:"server_endpoint"`
	ServerIP       string `json:"server_ip,omitempty"`
	GameNetCIDR    string `json:"game_net_cidr"`
	DNS            string `json:"dns,omitempty"`
	Provisioned    bool   `json:"provisioned"`
	LastSyncError  string `json:"last_sync_error,omitempty"`
}

type challengeRestartVote struct {
	Votes          int  `json:"votes"`
	Threshold      int  `json:"threshold"`
	EligibleVoters int  `json:"eligible_voters"`
	HasVoted       bool `json:"has_voted"`
	CanVote        bool `json:"can_vote"`
}

type challengeServiceRuntime struct {
	Status         string `json:"status"`
	Backend        string `json:"backend"`
	TargetID       string `json:"target_id,omitempty"`
	InstanceID     string `json:"instance_id"`
	ConnectionInfo string `json:"connection_info,omitempty"`
	CreatedAt      string `json:"created_at"`
}

type challengeRestartEvent struct {
	TriggerMode string `json:"trigger_mode"`
	Result      string `json:"result"`
	Message     string `json:"message,omitempty"`
	Actor       string `json:"actor,omitempty"`
	CreatedAt   string `json:"created_at"`
}

func (s *Server) handleChallengeWorkflow(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	var challengeType string
	var visible bool
	if err := s.db.QueryRow(
		`SELECT challenge_type, is_visible FROM challenges WHERE id=?`,
		challengeID,
	).Scan(&challengeType, &visible); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if !visible {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
	}

	userID := getUserID(c)
	ownerID := s.getTeamIDForUser(userID)
	resp := challengeWorkflowResponse{
		ChallengeID:   challengeID,
		ChallengeType: challengeType,
		OwnerID:       ownerID,
		TeamMode:      s.cfg.CTF.TeamMode,
		VPNEnabled:    s.cfg.AD.VPN.Enabled,
	}
	if s.adEngine != nil {
		resp.CurrentRound = s.adEngine.CurrentRound()
	}
	resp.CanAdminRestart = getUserRole(c) == "admin"
	if resp.VPNEnabled && ownerID > 0 {
		ownerType := "user"
		if s.cfg.CTF.TeamMode {
			ownerType = "team"
		}
		resp.VPNDownload = "/api/ad/vpn"
		resp.VPNSyncURL = "/api/ad/vpn/sync"
		if peer, teamIP, err := s.ensureADVPNPeer(ownerID, false); err == nil || peer.ID > 0 {
			resp.VPN = &challengeWorkflowVPN{
				OwnerType:      ownerType,
				OwnerID:        ownerID,
				ClientAddress:  teamIP,
				AllowedSubnet:  peer.AllowedIP,
				ServerEndpoint: s.cfg.AD.VPN.ServerEndpoint,
				ServerIP:       s.cfg.AD.VPN.ServerIP,
				GameNetCIDR:    s.cfg.AD.VPN.GameNetCIDR,
				DNS:            s.cfg.AD.VPN.DNS,
				Provisioned:    peer.Provisioned,
				LastSyncError:  peer.LastSyncError,
			}
		}
	}

	switch strings.TrimSpace(strings.ToLower(challengeType)) {
	case "attack_defence_attack":
		resp.Languages = []string{"python3", "bash"}
		resp.Sploits = s.listChallengeSploits(ownerID, challengeID)
		resp.ServiceInstance = s.latestSharedServiceInstance(challengeID)
		if len(resp.Sploits) == 0 {
			resp.Sploits = []models.Sploit{}
		}
		resp.Notes = []string{
			"Upload one exploit per approach. The platform runs enabled exploits automatically each round.",
			"Exploit scripts receive HOST, PORT, and TARGET environment variables.",
		}
	case "attack_defence_defense":
		resp.ServiceStatus = s.latestChallengeServiceStatus(challengeID, ownerID)
		resp.ServiceRounds = s.challengeServiceHistory(challengeID, ownerID, 8)
		if len(resp.ServiceRounds) == 0 {
			resp.ServiceRounds = []models.ADServiceStatus{}
		}
		resp.Notes = []string{
			"Keep the service reachable over VPN and watch checker rounds here.",
			"Score is based on weighted defense checks from checker_config.",
		}
	case "pentest":
		resp.RestartVote = s.restartVoteState(challengeID, userID)
		resp.ServiceInstance = s.latestSharedServiceInstance(challengeID)
		resp.RestartEvents = s.restartEvents(challengeID, 8)
		if len(resp.RestartEvents) == 0 {
			resp.RestartEvents = []challengeRestartEvent{}
		}
		resp.Notes = []string{
			"Use the VPN profile to reach the target network.",
			"Submit user, root, and additional flags through the normal challenge submit flow.",
			"If the shared service is broken, players can vote to restart it.",
		}
	default:
		resp.Notes = []string{}
	}

	return c.JSON(http.StatusOK, resp)
}

func (s *Server) handleChallengeWorkflowRestartVote(c echo.Context) error {
	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	userID := getUserID(c)

	var challengeType, deployType string
	var visible bool
	err = s.db.QueryRow(`SELECT challenge_type, deploy_type, is_visible FROM challenges WHERE id=?`, challengeID).
		Scan(&challengeType, &deployType, &visible)
	if err == sql.ErrNoRows || !visible {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	if challengeType != "pentest" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "restart voting is only available for pentest challenges"})
	}
	if canonicalDeployType(deployType, challengeType) != "always_on" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "restart voting is only available for always-on services"})
	}

	if err := s.recordRestartVote(challengeID, userID); err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusBadRequest, map[string]string{"error": "join a team before voting to restart this service"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "internal error"})
	}

	state := s.restartVoteState(challengeID, userID)
	if state == nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "failed to compute restart vote state"})
	}
	if state.Votes < state.Threshold {
		return c.JSON(http.StatusOK, map[string]any{
			"message":      "restart vote recorded",
			"restart_vote": state,
			"restarted":    false,
		})
	}

	if err := s.restartAlwaysOnChallenge(challengeID); err != nil {
		s.recordRestartEvent(challengeID, userID, "vote_threshold", "failed", err.Error())
		return c.JSON(http.StatusServiceUnavailable, map[string]any{
			"error":        "restart failed",
			"restart_vote": state,
			"restarted":    false,
		})
	}

	_, _ = s.db.Exec(`DELETE FROM challenge_restart_votes WHERE challenge_id=?`, challengeID)
	s.recordRestartEvent(challengeID, userID, "vote_threshold", "success", "restart threshold reached")
	return c.JSON(http.StatusOK, map[string]any{
		"message": "restart threshold reached and the service restart was triggered",
		"restart_vote": &challengeRestartVote{
			Votes:          0,
			Threshold:      state.Threshold,
			EligibleVoters: state.EligibleVoters,
			HasVoted:       false,
			CanVote:        true,
		},
		"restarted": true,
	})
}

func (s *Server) handleChallengeWorkflowAdminRestart(c echo.Context) error {
	if getUserRole(c) != "admin" {
		return c.JSON(http.StatusForbidden, map[string]string{"error": "admin access required"})
	}

	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	var challengeType, deployType string
	var visible bool
	err = s.db.QueryRow(`SELECT challenge_type, deploy_type, is_visible FROM challenges WHERE id=?`, challengeID).
		Scan(&challengeType, &deployType, &visible)
	if err == sql.ErrNoRows || !visible {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if challengeType != "pentest" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "admin restart is only available for pentest challenges"})
	}
	if canonicalDeployType(deployType, challengeType) != "always_on" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "admin restart is only available for always-on services"})
	}

	if err := s.restartAlwaysOnChallenge(challengeID); err != nil {
		s.recordRestartEvent(challengeID, getUserID(c), "admin", "failed", err.Error())
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "restart failed"})
	}
	_, _ = s.db.Exec(`DELETE FROM challenge_restart_votes WHERE challenge_id=?`, challengeID)
	s.recordRestartEvent(challengeID, getUserID(c), "admin", "success", "service restart triggered by admin")
	return c.JSON(http.StatusOK, map[string]string{"message": "service restart triggered"})
}

func (s *Server) handleChallengeWorkflowCheck(c echo.Context) error {
	if s.adEngine == nil {
		return c.JSON(http.StatusServiceUnavailable, map[string]string{"error": "attack-defense engine is not running"})
	}

	challengeID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}

	ownerID := s.getTeamIDForUser(getUserID(c))
	if ownerID == 0 {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "join a team before using defense checks"})
	}

	var challengeType string
	var checkerConfig string
	var visible bool
	err = s.db.QueryRow(
		`SELECT challenge_type, checker_config, is_visible FROM challenges WHERE id=?`,
		challengeID,
	).Scan(&challengeType, &checkerConfig, &visible)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if !visible {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "challenge not found"})
	}
	if challengeType != "attack_defence_defense" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "manual checks are only available for defense challenges"})
	}

	var connectionInfo string
	err = s.db.QueryRow(`
		SELECT connection_info
		FROM instances
		WHERE challenge_id=? AND status='running' AND team_id=?
		ORDER BY id DESC
		LIMIT 1
	`, challengeID, ownerID).Scan(&connectionInfo)
	if err != nil {
		if err == sql.ErrNoRows {
			return c.JSON(http.StatusConflict, map[string]string{"error": "no running service instance found for this team"})
		}
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	st, err := s.adEngine.ProbeService(challengeID, ownerID, challengeType, checkerConfig, connectionInfo)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "probe failed"})
	}
	st.MaxScore = adServiceMaxScore(checkerConfig)
	return c.JSON(http.StatusOK, st)
}

func (s *Server) listChallengeSploits(ownerID, challengeID int64) []models.Sploit {
	if ownerID == 0 {
		return []models.Sploit{}
	}
	rows, err := s.db.Query(
		`SELECT id, team_id, challenge_id, bucket_key, name, language, enabled, created_at, last_run_at
		 FROM ad_sploits
		 WHERE team_id=? AND challenge_id=?
		 ORDER BY id DESC`,
		ownerID, challengeID,
	)
	if err != nil {
		return []models.Sploit{}
	}
	defer rows.Close()

	var sploits []models.Sploit
	for rows.Next() {
		var sp models.Sploit
		if err := rows.Scan(&sp.ID, &sp.TeamID, &sp.ChallengeID, &sp.BucketKey, &sp.Name, &sp.Language, &sp.Enabled, &sp.CreatedAt, &sp.LastRunAt); err != nil {
			continue
		}
		sploits = append(sploits, sp)
	}
	return sploits
}

func (s *Server) latestChallengeServiceStatus(challengeID, ownerID int64) *models.ADServiceStatus {
	if ownerID == 0 {
		return nil
	}
	var st models.ADServiceStatus
	var checkerConfig string
	err := s.db.QueryRow(`
		SELECT svc.challenge_id, ch.name, ch.checker_config, svc.team_id, svc.status, svc.round, svc.score, svc.checked_at
		FROM ad_services svc
		JOIN challenges ch ON ch.id = svc.challenge_id
		WHERE svc.challenge_id=? AND svc.team_id=?
		ORDER BY svc.round DESC
		LIMIT 1
	`, challengeID, ownerID).Scan(&st.ChallengeID, &st.ChallengeName, &checkerConfig, &st.TeamID, &st.Status, &st.Round, &st.Score, &st.CheckedAt)
	if err != nil {
		return nil
	}
	st.MaxScore = adServiceMaxScore(checkerConfig)
	return &st
}

func (s *Server) challengeServiceHistory(challengeID, ownerID int64, limit int) []models.ADServiceStatus {
	if ownerID == 0 || limit <= 0 {
		return []models.ADServiceStatus{}
	}
	rows, err := s.db.Query(`
		SELECT svc.challenge_id, ch.name, ch.checker_config, svc.team_id, svc.status, svc.round, svc.score, svc.checked_at
		FROM ad_services svc
		JOIN challenges ch ON ch.id = svc.challenge_id
		WHERE svc.challenge_id=? AND svc.team_id=?
		ORDER BY svc.round DESC
		LIMIT ?
	`, challengeID, ownerID, limit)
	if err != nil {
		return []models.ADServiceStatus{}
	}
	defer rows.Close()

	var history []models.ADServiceStatus
	for rows.Next() {
		var st models.ADServiceStatus
		var checkerConfig string
		if err := rows.Scan(&st.ChallengeID, &st.ChallengeName, &checkerConfig, &st.TeamID, &st.Status, &st.Round, &st.Score, &st.CheckedAt); err != nil {
			continue
		}
		st.MaxScore = adServiceMaxScore(checkerConfig)
		history = append(history, st)
	}
	return history
}

func (s *Server) restartVoteState(challengeID, userID int64) *challengeRestartVote {
	eligible := s.restartEligibleVoters()
	if eligible <= 0 {
		eligible = 1
	}
	threshold := eligible / 2
	if eligible%2 != 0 {
		threshold++
	}
	if threshold < 1 {
		threshold = 1
	}

	votes := 0
	hasVoted := false
	if s.cfg.CTF.TeamMode {
		teamID := s.getTeamIDForUser(userID)
		if teamID > 0 {
			_ = s.db.QueryRow(`SELECT COUNT(*) FROM challenge_restart_votes WHERE challenge_id=? AND team_id IS NOT NULL`, challengeID).Scan(&votes)
			var count int
			_ = s.db.QueryRow(`SELECT COUNT(*) FROM challenge_restart_votes WHERE challenge_id=? AND team_id=?`, challengeID, teamID).Scan(&count)
			hasVoted = count > 0
		}
	} else {
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM challenge_restart_votes WHERE challenge_id=? AND user_id IS NOT NULL`, challengeID).Scan(&votes)
		var count int
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM challenge_restart_votes WHERE challenge_id=? AND user_id=?`, challengeID, userID).Scan(&count)
		hasVoted = count > 0
	}

	return &challengeRestartVote{
		Votes:          votes,
		Threshold:      threshold,
		EligibleVoters: eligible,
		HasVoted:       hasVoted,
		CanVote:        !hasVoted,
	}
}

func (s *Server) restartEligibleVoters() int {
	var count int
	if s.cfg.CTF.TeamMode {
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM teams`).Scan(&count)
	} else {
		_ = s.db.QueryRow(`SELECT COUNT(*) FROM users WHERE banned=FALSE`).Scan(&count)
	}
	return count
}

func (s *Server) recordRestartVote(challengeID, userID int64) error {
	if s.cfg.CTF.TeamMode {
		teamID := s.getTeamIDForUser(userID)
		if teamID == 0 {
			return sql.ErrNoRows
		}
		_, err := s.db.Exec(`INSERT INTO challenge_restart_votes (challenge_id, team_id) VALUES (?, ?)`, challengeID, teamID)
		if err != nil && !strings.Contains(strings.ToLower(err.Error()), "unique") {
			return err
		}
		return nil
	}

	_, err := s.db.Exec(`INSERT INTO challenge_restart_votes (challenge_id, user_id) VALUES (?, ?)`, challengeID, userID)
	if err != nil && !strings.Contains(strings.ToLower(err.Error()), "unique") {
		return err
	}
	return nil
}

func (s *Server) restartAlwaysOnChallenge(challengeID int64) error {
	var ch models.Challenge
	err := s.db.QueryRow(
		`SELECT id, challenge_type, deploy_type, deploy_backend, deploy_config, image, vm_template FROM challenges WHERE id=? AND is_visible`,
		challengeID,
	).Scan(&ch.ID, &ch.ChallengeType, &ch.DeployType, &ch.DeployBackend, &ch.DeployConfig, &ch.Image, &ch.VMTemplate)
	if err == sql.ErrNoRows {
		return fmt.Errorf("challenge not found")
	}
	if err != nil {
		return fmt.Errorf("db error")
	}

	ch.DeployType = canonicalDeployType(ch.DeployType, ch.ChallengeType)
	if ch.DeployType != "always_on" {
		return fmt.Errorf("challenge is not configured as always-on")
	}

	var preservedConnInfo string
	if current, ok := s.findRunningInstance(challengeID, 0, 0, ch.DeployType); ok {
		preservedConnInfo = current.ConnectionInfo
		if current.Backend != "" && current.Backend != "no_deploy" {
			targetName := current.TargetID
			if targetName == "" {
				targetName = current.Backend
			}
			back, err := s.deployer.Get(targetName)
			if err != nil {
				return fmt.Errorf("deployer backend not available: %w", err)
			}
			if err := back.Destroy(context.Background(), current.InstanceID); err != nil {
				return fmt.Errorf("failed to stop running service: %w", err)
			}
		}
		_, _ = s.db.Exec(`UPDATE instances SET status='stopped' WHERE id=?`, current.ID)
	}

	var deployConfig map[string]any
	if ch.DeployConfig != "" && ch.DeployConfig != "{}" {
		_ = json.Unmarshal([]byte(ch.DeployConfig), &deployConfig)
	}
	if deployConfig == nil {
		deployConfig = map[string]any{}
	}
	requestedCPU, requestedMemMB := parseRequestedResources(deployConfig)
	if strings.TrimSpace(ch.DeployBackend) == "" || strings.TrimSpace(ch.DeployBackend) == "no_deploy" {
		if preservedConnInfo == "" {
			preservedConnInfo = ch.ConnectionInfo
		}
		if strings.TrimSpace(preservedConnInfo) == "" {
			preservedConnInfo = `{}`
		}
		instanceID := fmt.Sprintf("manual-restart-%d", challengeID)
		_, err = s.db.InsertGetID(
			`INSERT INTO instances (challenge_id, user_id, team_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag, reserved_cpu_mil, reserved_memory_mb, expires_at, status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
			challengeID, nil, nil, ch.DeployType, "no_deploy", "manual", instanceID, preservedConnInfo, "", int(requestedCPU*1000), requestedMemMB, nil, "running",
		)
		if err != nil {
			return fmt.Errorf("db error saving restarted instance: %w", err)
		}
		return nil
	}
	selected, back, err := s.resolveDeployerSelection(ch.DeployBackend, requestedCPU, requestedMemMB)
	if err != nil {
		return fmt.Errorf("deployer backend not available: %w", err)
	}
	s.injectRuntimeEnv(deployConfig, &ch, 0, 0, "")

	inst, err := back.Deploy(context.Background(), deployer.DeployRequest{
		ChallengeID:  ch.ID,
		Image:        ch.Image,
		VMTemplate:   ch.VMTemplate,
		DeployConfig: deployConfig,
		DeployType:   ch.DeployType,
		Backend:      selected.Type,
		InstanceTTL:  s.cfg.Deployer.InstanceTTL,
	})
	if err != nil {
		return fmt.Errorf("deploy failed: %w", err)
	}

	connInfo := inst.ConnectionInfo
	if connInfo == "" {
		connInfo = "{}"
	}
	_, err = s.db.InsertGetID(
		`INSERT INTO instances (challenge_id, user_id, team_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag, reserved_cpu_mil, reserved_memory_mb, expires_at, status) VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?)`,
		challengeID, nil, nil, ch.DeployType, selected.Type, selected.ID, inst.InstanceID, connInfo, "", int(requestedCPU*1000), requestedMemMB, inst.ExpiresAt, "running",
	)
	if err != nil {
		return fmt.Errorf("db error saving restarted instance: %w", err)
	}
	return nil
}

func (s *Server) latestSharedServiceInstance(challengeID int64) *challengeServiceRuntime {
	var inst models.Instance
	var userID sql.NullInt64
	var teamID sql.NullInt64
	var expiresAt sql.NullTime
	err := s.db.QueryRow(`
		SELECT id, challenge_id, user_id, team_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag,
		       reserved_cpu_mil, reserved_memory_mb, created_at, expires_at, status
		FROM instances
		WHERE challenge_id=? AND status='running' AND user_id IS NULL AND team_id IS NULL
		ORDER BY id DESC
		LIMIT 1
	`, challengeID).Scan(
		&inst.ID, &inst.ChallengeID, &userID, &teamID, &inst.InstanceType, &inst.Backend, &inst.TargetID,
		&inst.InstanceID, &inst.ConnectionInfo, &inst.DynamicFlag, &inst.ReservedCPUMil, &inst.ReservedMemMB,
		&inst.CreatedAt, &expiresAt, &inst.Status,
	)
	if err != nil {
		return nil
	}
	if userID.Valid {
		inst.UserID = &userID.Int64
	}
	if teamID.Valid {
		inst.TeamID = &teamID.Int64
	}
	if expiresAt.Valid {
		inst.ExpiresAt = &expiresAt.Time
	}
	return &challengeServiceRuntime{
		Status:         inst.Status,
		Backend:        inst.Backend,
		TargetID:       inst.TargetID,
		InstanceID:     inst.InstanceID,
		ConnectionInfo: inst.ConnectionInfo,
		CreatedAt:      inst.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
}

func (s *Server) restartEvents(challengeID int64, limit int) []challengeRestartEvent {
	if limit <= 0 {
		return []challengeRestartEvent{}
	}
	rows, err := s.db.Query(`
		SELECT evt.trigger_mode, evt.result, evt.message, COALESCE(u.username, ''), evt.created_at
		FROM challenge_restart_events evt
		LEFT JOIN users u ON u.id = evt.triggered_by_user_id
		WHERE evt.challenge_id=?
		ORDER BY evt.id DESC
		LIMIT ?
	`, challengeID, limit)
	if err != nil {
		return []challengeRestartEvent{}
	}
	defer rows.Close()

	history := make([]challengeRestartEvent, 0, limit)
	for rows.Next() {
		var item challengeRestartEvent
		var createdAt time.Time
		if err := rows.Scan(&item.TriggerMode, &item.Result, &item.Message, &item.Actor, &createdAt); err != nil {
			continue
		}
		item.CreatedAt = createdAt.Format("2006-01-02T15:04:05Z07:00")
		history = append(history, item)
	}
	return history
}

func (s *Server) recordRestartEvent(challengeID, triggeredByUserID int64, triggerMode, result, message string) {
	var actorID any
	if triggeredByUserID > 0 {
		actorID = triggeredByUserID
	}
	_, _ = s.db.Exec(
		`INSERT INTO challenge_restart_events (challenge_id, triggered_by_user_id, trigger_mode, result, message) VALUES (?, ?, ?, ?, ?)`,
		challengeID, actorID, triggerMode, result, message,
	)
}
