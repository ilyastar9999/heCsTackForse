package api

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"

	"github.com/ilyastar9999/heCsTackForse/internal/ad"
	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

// ────────────────────────────────────────────────────────────────────────────
// AD status
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADStatus(w http.ResponseWriter, r *http.Request) {
	round := int64(0)
	if s.adEngine != nil {
		round = s.adEngine.CurrentRound()
	}
	jsonResponse(w, http.StatusOK, map[string]any{
		"round":          round,
		"round_duration": s.cfg.AD.RoundDuration,
		"flag_submit_url": s.cfg.AD.FlagSubmitURL,
		"vpn_enabled":    s.cfg.AD.VPN.Enabled,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// AD scoreboard — attack/defence points per team
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADScoreboard(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT t.id, t.name,
		       COALESCE(SUM(CASE WHEN svc.status='up' THEN 1 ELSE 0 END), 0) AS defence_pts,
		       COALESCE(COUNT(DISTINCT sr.id), 0)                             AS attack_pts
		FROM teams t
		LEFT JOIN ad_services svc ON svc.team_id = t.id
		LEFT JOIN ad_sploit_results sr ON sr.flags_submitted > 0
		          AND EXISTS (
		              SELECT 1 FROM ad_sploits sp
		              WHERE sp.id = sr.sploit_id AND sp.team_id = t.id
		          )
		GROUP BY t.id, t.name
		ORDER BY (COALESCE(SUM(CASE WHEN svc.status='up' THEN 1 ELSE 0 END), 0) +
		          COALESCE(COUNT(DISTINCT sr.id), 0)) DESC
	`)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	type entry struct {
		TeamID     int64  `json:"team_id"`
		TeamName   string `json:"team_name"`
		DefencePts int64  `json:"defence_pts"`
		AttackPts  int64  `json:"attack_pts"`
		Total      int64  `json:"total"`
	}
	var board []entry
	for rows.Next() {
		var e entry
		if err := rows.Scan(&e.TeamID, &e.TeamName, &e.DefencePts, &e.AttackPts); err != nil {
			continue
		}
		e.Total = e.DefencePts + e.AttackPts
		board = append(board, e)
	}
	if board == nil {
		board = []entry{}
	}
	jsonResponse(w, http.StatusOK, board)
}

// ────────────────────────────────────────────────────────────────────────────
// Service status grid
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADServices(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query(`
		SELECT svc.challenge_id, svc.team_id, svc.status, svc.round, svc.score, svc.checked_at
		FROM ad_services svc
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
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var statuses []models.ADServiceStatus
	for rows.Next() {
		var st models.ADServiceStatus
		if err := rows.Scan(&st.ChallengeID, &st.TeamID, &st.Status, &st.Round, &st.Score, &st.CheckedAt); err != nil {
			continue
		}
		statuses = append(statuses, st)
	}
	if statuses == nil {
		statuses = []models.ADServiceStatus{}
	}
	jsonResponse(w, http.StatusOK, statuses)
}

// ────────────────────────────────────────────────────────────────────────────
// VPN config download
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADGetVPN(w http.ResponseWriter, r *http.Request) {
	if !s.cfg.AD.VPN.Enabled {
		jsonError(w, "VPN not enabled", http.StatusNotFound)
		return
	}
	teamID := s.getTeamIDForUser(getUserID(r))
	if teamID == 0 {
		jsonError(w, "you must be in a team to get VPN config", http.StatusBadRequest)
		return
	}

	// Fetch or create VPN peer for this team
	var peer models.VPNPeer
	err := s.db.QueryRow(
		`SELECT id, team_id, private_key, public_key, allowed_ip, created_at FROM ad_vpn_peers WHERE team_id=?`,
		teamID,
	).Scan(&peer.ID, &peer.TeamID, &peer.PrivateKey, &peer.PublicKey, &peer.AllowedIP, &peer.CreatedAt)

	if err == sql.ErrNoRows {
		// Generate new keypair
		privB64, pubB64, genErr := ad.GenerateWireGuardKeys()
		if genErr != nil {
			jsonError(w, "failed to generate VPN keys: "+genErr.Error(), http.StatusInternalServerError)
			return
		}
		// Assign team IP from config: base + teamID + ".1/24"
		base := s.cfg.AD.VPN.TeamSubnetBase
		if base == "" {
			base = "10.8."
		}
		teamIP := fmt.Sprintf("%s%d.1/24", base, teamID)
		allowedIP := fmt.Sprintf("%s%d.0/24", base, teamID)

		newID, insErr := s.db.InsertGetID(
			`INSERT INTO ad_vpn_peers (team_id, private_key, public_key, allowed_ip) VALUES (?, ?, ?, ?)`,
			teamID, privB64, pubB64, allowedIP,
		)
		if insErr != nil {
			jsonError(w, "db error: "+insErr.Error(), http.StatusInternalServerError)
			return
		}
		peer.TeamID = teamID
		peer.PrivateKey = privB64
		peer.PublicKey = pubB64
		peer.AllowedIP = teamIP
		peer.ID = newID
	} else if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}

	cfg := s.cfg.AD.VPN
	base := cfg.TeamSubnetBase
	if base == "" {
		base = "10.8."
	}
	teamIP := fmt.Sprintf("%s%d.1/24", base, teamID)

	conf := ad.BuildClientConfig(
		peer.PrivateKey,
		teamIP,
		cfg.ServerPublicKey,
		cfg.ServerEndpoint,
		cfg.ServerIP,
		cfg.GameNetCIDR,
		cfg.DNS,
	)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="team%d-wg.conf"`, teamID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(conf))
}

// ────────────────────────────────────────────────────────────────────────────
// Sploit CRUD
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADListSploits(w http.ResponseWriter, r *http.Request) {
	teamID := s.getTeamIDForUser(getUserID(r))
	if teamID == 0 {
		jsonError(w, "you must be in a team", http.StatusBadRequest)
		return
	}
	rows, err := s.db.Query(
		`SELECT id, team_id, challenge_id, name, language, enabled, created_at, last_run_at
		 FROM ad_sploits WHERE team_id=? ORDER BY id`,
		teamID,
	)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var sploits []models.Sploit
	for rows.Next() {
		var sp models.Sploit
		if err := rows.Scan(&sp.ID, &sp.TeamID, &sp.ChallengeID, &sp.Name,
			&sp.Language, &sp.Enabled, &sp.CreatedAt, &sp.LastRunAt); err != nil {
			continue
		}
		sploits = append(sploits, sp)
	}
	if sploits == nil {
		sploits = []models.Sploit{}
	}
	jsonResponse(w, http.StatusOK, sploits)
}

func (s *Server) handleADCreateSploit(w http.ResponseWriter, r *http.Request) {
	teamID := s.getTeamIDForUser(getUserID(r))
	if teamID == 0 {
		jsonError(w, "you must be in a team", http.StatusBadRequest)
		return
	}
	var req struct {
		ChallengeID int64  `json:"challenge_id"`
		Name        string `json:"name"`
		Language    string `json:"language"`
		Script      string `json:"script"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}
	if req.Script == "" || req.Name == "" {
		jsonError(w, "name and script are required", http.StatusBadRequest)
		return
	}
	if req.Language == "" {
		req.Language = "python3"
	}
	id, err := s.db.InsertGetID(
		`INSERT INTO ad_sploits (team_id, challenge_id, name, language, script, enabled) VALUES (?,?,?,?,?,TRUE)`,
		teamID, req.ChallengeID, req.Name, req.Language, req.Script,
	)
	if err != nil {
		jsonError(w, "db error: "+err.Error(), http.StatusInternalServerError)
		return
	}
	jsonResponse(w, http.StatusCreated, map[string]any{"id": id, "message": "sploit created"})
}

func (s *Server) handleADUpdateSploit(w http.ResponseWriter, r *http.Request) {
	sploitID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	teamID := s.getTeamIDForUser(getUserID(r))

	var req struct {
		Name     string `json:"name"`
		Language string `json:"language"`
		Script   string `json:"script"`
		Enabled  *bool  `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", http.StatusBadRequest)
		return
	}

	// Verify ownership
	var ownerTeam int64
	if err := s.db.QueryRow(`SELECT team_id FROM ad_sploits WHERE id=?`, sploitID).Scan(&ownerTeam); err != nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if ownerTeam != teamID && getUserRole(r) != "admin" {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
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
	if req.Name != "" {
		_, _ = s.db.Exec(`UPDATE ad_sploits SET name=? WHERE id=?`, req.Name, sploitID)
	}
	if req.Language != "" {
		_, _ = s.db.Exec(`UPDATE ad_sploits SET language=? WHERE id=?`, req.Language, sploitID)
	}
	jsonResponse(w, http.StatusOK, map[string]string{"message": "updated"})
}

func (s *Server) handleADDeleteSploit(w http.ResponseWriter, r *http.Request) {
	sploitID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	teamID := s.getTeamIDForUser(getUserID(r))
	var ownerTeam int64
	if err := s.db.QueryRow(`SELECT team_id FROM ad_sploits WHERE id=?`, sploitID).Scan(&ownerTeam); err != nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if ownerTeam != teamID && getUserRole(r) != "admin" {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}
	_, _ = s.db.Exec(`DELETE FROM ad_sploit_results WHERE sploit_id=?`, sploitID)
	_, _ = s.db.Exec(`DELETE FROM ad_sploits WHERE id=?`, sploitID)
	jsonResponse(w, http.StatusOK, map[string]string{"message": "deleted"})
}

func (s *Server) handleADSploitResults(w http.ResponseWriter, r *http.Request) {
	sploitID, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		jsonError(w, "invalid id", http.StatusBadRequest)
		return
	}
	teamID := s.getTeamIDForUser(getUserID(r))
	var ownerTeam int64
	if err := s.db.QueryRow(`SELECT team_id FROM ad_sploits WHERE id=?`, sploitID).Scan(&ownerTeam); err != nil {
		jsonError(w, "not found", http.StatusNotFound)
		return
	}
	if ownerTeam != teamID && getUserRole(r) != "admin" {
		jsonError(w, "forbidden", http.StatusForbidden)
		return
	}
	rows, err := s.db.Query(`
		SELECT id, sploit_id, target_team_id, round, flags_captured, flags_submitted, error, ran_at
		FROM ad_sploit_results WHERE sploit_id=? ORDER BY round DESC LIMIT 200`,
		sploitID,
	)
	if err != nil {
		jsonError(w, "db error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()
	var results []models.SploitResult
	for rows.Next() {
		var res models.SploitResult
		if err := rows.Scan(&res.ID, &res.SploitID, &res.TargetTeamID, &res.Round,
			&res.FlagsCaptured, &res.FlagsSubmitted, &res.Error, &res.RanAt); err != nil {
			continue
		}
		results = append(results, res)
	}
	if results == nil {
		results = []models.SploitResult{}
	}
	jsonResponse(w, http.StatusOK, results)
}

// ────────────────────────────────────────────────────────────────────────────
// Manual flag submission proxy (player submits a captured flag directly)
// ────────────────────────────────────────────────────────────────────────────

func (s *Server) handleADSubmitFlag(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Flags []string `json:"flags"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Flags) == 0 {
		jsonError(w, "provide flags array", http.StatusBadRequest)
		return
	}
	if s.cfg.AD.FlagSubmitURL == "" {
		jsonError(w, "central flag submitter not configured", http.StatusServiceUnavailable)
		return
	}
	// Store as ad_flags for record-keeping
	userID := getUserID(r)
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
		jsonError(w, "failed to build request", http.StatusInternalServerError)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if s.cfg.AD.FlagSubmitKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+s.cfg.AD.FlagSubmitKey)
		httpReq.Header.Set("X-Team-Token", s.cfg.AD.FlagSubmitKey)
	}
	client := &http.Client{}
	resp, err := client.Do(httpReq)
	if err != nil {
		jsonError(w, "flag submitter unreachable: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	jsonResponse(w, http.StatusOK, map[string]any{
		"submitted": len(req.Flags),
		"server_status": resp.StatusCode,
	})
}

// ────────────────────────────────────────────────────────────────────────────
// Helpers
// ────────────────────────────────────────────────────────────────────────────

// getTeamIDForUser looks up which team the user belongs to.
func (s *Server) getTeamIDForUser(userID int64) int64 {
	var teamID int64
	_ = s.db.QueryRow(
		`SELECT team_id FROM team_members WHERE user_id=? LIMIT 1`, userID,
	).Scan(&teamID)
	return teamID
}
