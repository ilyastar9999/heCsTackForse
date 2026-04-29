package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/ad"
	"github.com/ilyastar9999/heCsTackForse/internal/config"
	"github.com/ilyastar9999/heCsTackForse/internal/db"
	"github.com/ilyastar9999/heCsTackForse/internal/deployer"
)

func newTestServer(t *testing.T) (*Server, *db.DB) {
	t.Helper()
	database, err := db.New("sqlite", filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatalf("db.New: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	cfg := &config.Config{}
	cfg.Server.SecretKey = "test-secret-key-with-enough-entropy"
	cfg.Server.StaticDir = "../../frontend"
	cfg.CTF.Name = "Test CTF"
	cfg.CTF.Mode = "ctf"
	cfg.CTF.RegistrationOpen = true
	cfg.CTF.Scoring = "static"
	cfg.CTF.FlagPrefix = "FLAG{"
	cfg.CTF.FlagSuffix = "}"
	cfg.Deployer.InstanceTTL = "1h"

	mgr := deployer.NewManager()
	mgr.Register(&deployer.NoDeployDeployer{})
	return NewServer(cfg, database, mgr, nil), database
}

func registerAndCookie(t *testing.T, srv *Server) *http.Cookie {
	t.Helper()
	body := bytes.NewBufferString(`{"username":"alice","email":"alice@example.com","password":"password123"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/register", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "token" {
			return cookie
		}
	}
	t.Fatal("token cookie not set")
	return nil
}

func loginCookie(t *testing.T, srv *Server, username, password string) *http.Cookie {
	t.Helper()
	body := bytes.NewBufferString(`{"username":"` + username + `","password":"` + password + `"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", rec.Code, rec.Body.String())
	}
	for _, cookie := range rec.Result().Cookies() {
		if cookie.Name == "token" {
			return cookie
		}
	}
	t.Fatal("login token cookie not set")
	return nil
}

func attachUserToTeam(t *testing.T, database *db.DB, userID int64, teamName string) int64 {
	t.Helper()
	teamID, err := database.InsertGetID(
		`INSERT INTO teams (name, invite_code, score) VALUES (?, ?, 0)`,
		teamName, teamName+"_CODE",
	)
	if err != nil {
		t.Fatalf("insert team: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO team_members (team_id, user_id) VALUES (?, ?)`, teamID, userID); err != nil {
		t.Fatalf("insert team member: %v", err)
	}
	return teamID
}

func vpnHookSuccessCommand() (string, []string) {
	if runtime.GOOS == "windows" {
		return "cmd", []string{"/c", "exit", "0"}
	}
	return "sh", []string{"-c", "exit 0"}
}

func TestAuthRequiredPagesAreNotPublic(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv)

	if _, err := database.Exec(
		`INSERT INTO pages (title, slug, content, draft, auth_required) VALUES (?, ?, ?, ?, ?)`,
		"Private", "private", "secret", false, true,
	); err != nil {
		t.Fatalf("insert page: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/pages/private", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated private page status = %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/pages", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list pages status = %d", rec.Code)
	}
	var publicPages []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &publicPages); err != nil {
		t.Fatalf("decode public pages: %v", err)
	}
	if len(publicPages) != 0 {
		t.Fatalf("private page leaked in public list: %+v", publicPages)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/pages/private", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("authenticated private page status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestInitialSetupCreatesAdminAndPersistsPublicConfig(t *testing.T) {
	srv, database := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("setup status = %d body=%s", rec.Code, rec.Body.String())
	}
	var status map[string]bool
	if err := json.Unmarshal(rec.Body.Bytes(), &status); err != nil {
		t.Fatalf("decode setup status: %v", err)
	}
	if !status["required"] {
		t.Fatal("expected setup to be required for empty database")
	}

	body := bytes.NewBufferString(`{
		"site_name":"Final CTF",
		"mode":"ad",
		"team_mode":true,
		"registration_open":false,
		"language":"ru",
		"admin_username":"admin",
		"admin_email":"admin@example.com",
		"admin_password":"password123"
	}`)
	req = httptest.NewRequest(http.MethodPost, "/api/setup", body)
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("setup status = %d body=%s", rec.Code, rec.Body.String())
	}
	if len(rec.Result().Cookies()) == 0 {
		t.Fatal("expected setup to set auth cookie")
	}

	var role string
	if err := database.QueryRow(`SELECT role FROM users WHERE username=?`, "admin").Scan(&role); err != nil {
		t.Fatalf("query admin: %v", err)
	}
	if role != "admin" {
		t.Fatalf("expected admin role, got %q", role)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/config", nil)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("config status = %d body=%s", rec.Code, rec.Body.String())
	}
	var cfg map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &cfg); err != nil {
		t.Fatalf("decode config: %v", err)
	}
	if cfg["name"] != "Final CTF" || cfg["mode"] != "ad" || cfg["language"] != "ru" || cfg["registration_open"] != false || cfg["team_mode"] != true {
		t.Fatalf("unexpected public config: %+v", cfg)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/auth/register", bytes.NewBufferString(`{"username":"bob","email":"bob@example.com","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("expected registration to remain closed, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestChallengeMaxAttemptsIsEnforced(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv)

	id, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, flag_type, deploy_type, deploy_backend, deploy_config, is_visible, max_attempts)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Limited", "desc", "misc", 100, "FLAG{ok}", "exact", "no_deploy", "", "{}", true, 1,
	)
	if err != nil {
		t.Fatalf("insert challenge: %v", err)
	}

	submit := func(flag string) *httptest.ResponseRecorder {
		body := bytes.NewBufferString(`{"flag":` + strconvQuote(flag) + `}`)
		req := httptest.NewRequest(http.MethodPost, "/api/challenges/"+strconvFormat(id)+"/submit", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	if rec := submit("FLAG{wrong}"); rec.Code != http.StatusOK {
		t.Fatalf("first wrong submit status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := submit("FLAG{still_wrong}"); rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second wrong submit status = %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestDynamicDeployFlagUsesInstanceContext(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv)

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Dynamic", "desc", "web", 100, "", "dynamic_deploy", "exact", "{}", "per_user", "docker", "{}", true,
	)
	if err != nil {
		t.Fatalf("insert challenge: %v", err)
	}

	if _, err := database.Exec(
		`INSERT INTO instances (challenge_id, user_id, instance_type, backend, instance_id, connection_info, dynamic_flag, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		challengeID, 1, "per_user", "docker", "inst-1", `{}`, "FLAG{dynamic_for_alice}", "running",
	); err != nil {
		t.Fatalf("insert instance: %v", err)
	}

	body := bytes.NewBufferString(`{"flag":"FLAG{dynamic_for_alice}"}`)
	req := httptest.NewRequest(http.MethodPost, "/api/challenges/"+strconvFormat(challengeID)+"/submit", body)
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("submit status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload["correct"] != true {
		t.Fatalf("expected dynamic flag to be accepted: %+v", payload)
	}
}

func TestPentestSupportsMultipleBuckets(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv)

	checkerConfig := `{"flags":[{"key":"user","label":"User","value":"FLAG{user_shell}","type":"exact","points":100},{"key":"root","label":"Root","value":"FLAG{root_shell}","type":"exact","points":400}]}`
	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Pentest", "desc", "pwn", 100, "", "pentest", "exact", checkerConfig, "no_deploy", "", "{}", true,
	)
	if err != nil {
		t.Fatalf("insert pentest challenge: %v", err)
	}

	submit := func(flag string) *httptest.ResponseRecorder {
		body := bytes.NewBufferString(`{"flag":` + strconvQuote(flag) + `}`)
		req := httptest.NewRequest(http.MethodPost, "/api/challenges/"+strconvFormat(challengeID)+"/submit", body)
		req.Header.Set("Content-Type", "application/json")
		req.AddCookie(cookie)
		rec := httptest.NewRecorder()
		srv.ServeHTTP(rec, req)
		return rec
	}

	if rec := submit("FLAG{user_shell}"); rec.Code != http.StatusOK {
		t.Fatalf("user bucket submit status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := submit("FLAG{root_shell}"); rec.Code != http.StatusOK {
		t.Fatalf("root bucket submit status = %d body=%s", rec.Code, rec.Body.String())
	}
	if rec := submit("FLAG{user_shell}"); rec.Code != http.StatusConflict {
		t.Fatalf("duplicate bucket submit status = %d body=%s", rec.Code, rec.Body.String())
	}

	var score int
	if err := database.QueryRow(`SELECT score FROM users WHERE id=?`, 1).Scan(&score); err != nil {
		t.Fatalf("query score: %v", err)
	}
	if score != 500 {
		t.Fatalf("unexpected accumulated pentest score = %d", score)
	}
}

func TestValidateADSploitTargetChecksBucketConfiguration(t *testing.T) {
	srv, database := newTestServer(t)

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"AD Attack", "desc", "ad", 100, "", "attack_defence_attack", "exact",
		`{"attack_buckets":[{"key":"rce","points":300},{"key":"sqli","points":150}]}`,
		"attack_defence", "docker", "{}", true,
	)
	if err != nil {
		t.Fatalf("insert AD attack challenge: %v", err)
	}

	if err := srv.validateADSploitTarget(challengeID, "rce"); err != nil {
		t.Fatalf("expected bucket to validate: %v", err)
	}
	if err := srv.validateADSploitTarget(challengeID, "unknown"); err == nil {
		t.Fatal("expected unknown bucket to fail validation")
	}
	if err := srv.validateADSploitTarget(0, "rce"); err == nil {
		t.Fatal("expected global sploit with bucket key to fail validation")
	}
}

func TestAdminDeployConfigPersistsAndAppliesSchedulerTargets(t *testing.T) {
	srv, database := newTestServer(t)
	registerAndCookie(t, srv)
	if _, err := database.Exec(`UPDATE users SET role='admin' WHERE id=1`); err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	cookie := loginCookie(t, srv, "alice", "password123")

	payload := `{
		"mode":"balanced",
		"targets":[
			{"id":"docker-a","type":"docker","enabled":true,"order":10,"max_instances":5,"max_cpu":4,"max_memory_mb":2048,"network":"ctf-net","challenges_dir":"./challenges","local_tag_prefix":"hecstack/"},
			{"id":"kube-a","type":"kubernetes","enabled":true,"order":20,"max_instances":10,"max_cpu":8,"max_memory_mb":4096,"namespace":"ctf","kubeconfig":"C:/kube/config"}
		]
	}`

	req := httptest.NewRequest(http.MethodPut, "/api/admin/deploy", bytes.NewBufferString(payload))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("set deploy config status = %d body=%s", rec.Code, rec.Body.String())
	}

	cfg := srv.deployer.SchedulerConfig()
	if cfg.Mode != "balanced" {
		t.Fatalf("unexpected scheduler mode: %+v", cfg)
	}
	if len(cfg.Targets) != 2 {
		t.Fatalf("unexpected target count: %+v", cfg.Targets)
	}

	var stored string
	if err := database.QueryRow(`SELECT value FROM ctf_settings WHERE key=?`, deploySchedulerSettingsKey).Scan(&stored); err != nil {
		t.Fatalf("query stored scheduler config: %v", err)
	}
	if stored == "" {
		t.Fatal("stored scheduler config is empty")
	}
}

func TestAdminDeployConfigValidatesTargetConnectionSettings(t *testing.T) {
	srv, database := newTestServer(t)
	registerAndCookie(t, srv)
	if _, err := database.Exec(`UPDATE users SET role='admin' WHERE id=1`); err != nil {
		t.Fatalf("promote admin: %v", err)
	}
	cookie := loginCookie(t, srv, "alice", "password123")

	req := httptest.NewRequest(http.MethodPut, "/api/admin/deploy", bytes.NewBufferString(`{
		"mode":"ordered",
		"targets":[{"id":"kube-bad","type":"kubernetes","enabled":true,"namespace":"ctf"}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected missing kubeconfig to fail, got %d body=%s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodPut, "/api/admin/deploy", bytes.NewBufferString(`{
		"mode":"ordered",
		"targets":[{"id":"docker-bad","type":"docker","enabled":true,"host":"tcp://docker.example:2376","tls_verify":true}]
	}`))
	req.Header.Set("Content-Type", "application/json")
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected missing docker cert_path to fail, got %d body=%s", rec.Code, rec.Body.String())
	}
}

func TestParseRequestedResourcesSupportsTopLevelAndNestedForms(t *testing.T) {
	cpu, mem := parseRequestedResources(map[string]any{
		"cpus":   "1.5",
		"memory": "2g",
	})
	if cpu != 1.5 || mem != 2048 {
		t.Fatalf("unexpected top-level resources cpu=%v mem=%d", cpu, mem)
	}

	cpu, mem = parseRequestedResources(map[string]any{
		"resources": map[string]any{
			"cpu":       "0.5",
			"memory_mb": 768,
		},
	})
	if cpu != 0.5 || mem != 768 {
		t.Fatalf("unexpected nested resources cpu=%v mem=%d", cpu, mem)
	}
}

func TestADCatalogExposesAttackBucketsIncludingLegacyFormat(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv)

	if _, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Legacy Attack", "desc", "ad", 100, "", "attack_defence_attack", "exact",
		`{"buckets":[{"key":"rce","label":"RCE","points":300}]}`,
		"attack_defence", "docker", "{}", true,
	); err != nil {
		t.Fatalf("insert legacy attack challenge: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/ad/catalog", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("catalog status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode catalog: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("unexpected catalog length: %+v", payload)
	}
	buckets, ok := payload[0]["buckets"].([]any)
	if !ok || len(buckets) != 1 {
		t.Fatalf("unexpected bucket payload: %+v", payload[0])
	}
}

func TestChallengeWorkflowForAttackReturnsChallengeScopedSploits(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv)

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Attack Box", "desc", "ad", 100, "", "attack_defence_attack", "exact",
		`{"attack_buckets":[{"key":"rce","points":300}]}`,
		"always_on", "docker", "{}", true,
	)
	if err != nil {
		t.Fatalf("insert attack challenge: %v", err)
	}
	otherChallengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Other Attack", "desc", "ad", 100, "", "attack_defence_attack", "exact",
		`{"attack_buckets":[{"key":"sqli","points":150}]}`,
		"always_on", "docker", "{}", true,
	)
	if err != nil {
		t.Fatalf("insert second attack challenge: %v", err)
	}

	if _, err := database.Exec(
		`INSERT INTO ad_sploits (team_id, challenge_id, bucket_key, name, language, script, enabled) VALUES (?, ?, ?, ?, ?, ?, TRUE)`,
		1, challengeID, "", "targeted", "python3", "print('ok')",
	); err != nil {
		t.Fatalf("insert sploit: %v", err)
	}
	if _, err := database.Exec(
		`INSERT INTO ad_sploits (team_id, challenge_id, bucket_key, name, language, script, enabled) VALUES (?, ?, ?, ?, ?, ?, TRUE)`,
		1, otherChallengeID, "", "other", "python3", "print('other')",
	); err != nil {
		t.Fatalf("insert second sploit: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/challenges/"+strconvFormat(challengeID)+"/workflow", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("workflow status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode workflow: %v", err)
	}
	sploits, ok := payload["sploits"].([]any)
	if !ok || len(sploits) != 1 {
		t.Fatalf("unexpected sploit payload: %+v", payload)
	}
}

func TestChallengeWorkflowForDefenseReturnsLatestServiceStatus(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv)
	srv.cfg.CTF.TeamMode = true
	srv.cfg.AD.VPN.Enabled = true
	srv.cfg.AD.VPN.ServerPublicKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	srv.cfg.AD.VPN.ServerEndpoint = "vpn.example.com:51820"
	srv.cfg.AD.VPN.ServerIP = "10.8.0.1"
	srv.cfg.AD.VPN.TeamSubnetBase = "10.8."
	srv.cfg.AD.VPN.GameNetCIDR = "10.10.0.0/16"
	srv.cfg.AD.VPN.DNS = "1.1.1.1"
	srv.cfg.AD.VPN.HookCommand, srv.cfg.AD.VPN.HookArgs = vpnHookSuccessCommand()
	teamID := attachUserToTeam(t, database, 1, "Blue Team")

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Defense Box", "desc", "ad", 100, "", "attack_defence_defense", "exact",
		`{"defense_checks":[{"key":"http","points":2},{"key":"db","points":3}]}`,
		"always_on", "docker", "{}", true,
	)
	if err != nil {
		t.Fatalf("insert defense challenge: %v", err)
	}
	if _, err := database.Exec(
		`INSERT INTO ad_services (challenge_id, team_id, status, round, score, checked_at) VALUES (?, ?, ?, ?, ?, CURRENT_TIMESTAMP)`,
		challengeID, teamID, "corrupt", 7, 3,
	); err != nil {
		t.Fatalf("insert service status: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/challenges/"+strconvFormat(challengeID)+"/workflow", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("workflow status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode workflow: %v", err)
	}
	serviceStatus, ok := payload["service_status"].(map[string]any)
	if !ok {
		t.Fatalf("missing service_status in payload: %+v", payload)
	}
	if serviceStatus["status"] != "corrupt" {
		t.Fatalf("unexpected service status: %+v", serviceStatus)
	}
	if serviceStatus["max_score"] != float64(5) {
		t.Fatalf("unexpected max_score: %+v", serviceStatus)
	}
	vpn, ok := payload["vpn"].(map[string]any)
	if !ok {
		t.Fatalf("missing vpn summary in payload: %+v", payload)
	}
	if vpn["server_endpoint"] != "vpn.example.com:51820" {
		t.Fatalf("unexpected vpn endpoint: %+v", vpn)
	}
	if vpn["provisioned"] != true {
		t.Fatalf("expected provisioned vpn peer: %+v", vpn)
	}
	if payload["vpn_sync_url"] != "/api/ad/vpn/sync" {
		t.Fatalf("unexpected vpn_sync_url: %+v", payload)
	}
}

func TestChallengeWorkflowCheckRunsInformationalDefenseProbe(t *testing.T) {
	srv, database := newTestServer(t)
	cfg := &config.Config{}
	cfg.AD.CheckerTimeout = "5s"
	cfg.AD.SploitDir = t.TempDir()
	cfg.CTF.FlagPrefix = "FLAG{"
	cfg.AD.RoundDuration = "5m"
	engine, err := ad.New(database, cfg)
	if err != nil {
		t.Fatalf("create ad engine: %v", err)
	}
	srv.adEngine = engine

	cookie := registerAndCookie(t, srv)

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Defense Probe", "desc", "ad", 100, "", "attack_defence_defense", "exact",
		`{}`,
		"always_on", "docker", "{}", true,
	)
	if err != nil {
		t.Fatalf("insert defense challenge: %v", err)
	}
	if _, err := database.Exec(
		`INSERT INTO instances (challenge_id, team_id, instance_type, backend, instance_id, connection_info, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		challengeID, 1, "always_on", "docker", "inst-1", `{"host":"127.0.0.1","port":"8080"}`, "running",
	); err != nil {
		t.Fatalf("insert instance: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/challenges/"+strconvFormat(challengeID)+"/workflow/check", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("workflow check status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode workflow check: %v", err)
	}
	if payload["status"] != "up" {
		t.Fatalf("unexpected probe status: %+v", payload)
	}
	if payload["score"] != float64(1) {
		t.Fatalf("unexpected probe scoring: %+v", payload)
	}
	if _, ok := payload["max_score"]; ok {
		t.Fatalf("unexpected probe scoring: %+v", payload)
	}
}

func TestPentestRestartVoteAppearsInWorkflowAndRecordsVote(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv)
	if _, err := database.Exec(`INSERT INTO users (username, email, password_hash, role) VALUES (?, ?, ?, ?)`, "bob", "bob@example.com", "hash", "user"); err != nil {
		t.Fatalf("insert user bob: %v", err)
	}
	if _, err := database.Exec(`INSERT INTO users (username, email, password_hash, role) VALUES (?, ?, ?, ?)`, "carol", "carol@example.com", "hash", "user"); err != nil {
		t.Fatalf("insert user carol: %v", err)
	}

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Pentest Service", "desc", "pwn", 100, "", "pentest", "exact", `{}`, "always_on", "no_deploy", "{}", true,
	)
	if err != nil {
		t.Fatalf("insert pentest challenge: %v", err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/challenges/"+strconvFormat(challengeID)+"/workflow", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("workflow status = %d body=%s", rec.Code, rec.Body.String())
	}

	var workflow map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &workflow); err != nil {
		t.Fatalf("decode workflow: %v", err)
	}
	restartVote, ok := workflow["restart_vote"].(map[string]any)
	if !ok {
		t.Fatalf("restart_vote missing from workflow: %+v", workflow)
	}
	if restartVote["threshold"] != float64(2) {
		t.Fatalf("unexpected threshold: %+v", restartVote)
	}

	req = httptest.NewRequest(http.MethodPost, "/api/challenges/"+strconvFormat(challengeID)+"/workflow/restart-vote", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("restart vote status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode restart vote: %v", err)
	}
	state, ok := payload["restart_vote"].(map[string]any)
	if !ok {
		t.Fatalf("restart_vote response missing: %+v", payload)
	}
	if state["votes"] != float64(1) || payload["restarted"] == true {
		t.Fatalf("unexpected restart vote state: %+v", payload)
	}
}

func TestAdminRestartForPentestAlwaysOnReplacesManualInstance(t *testing.T) {
	srv, database := newTestServer(t)
	cookie := registerAndCookie(t, srv) // first user is admin
	if _, err := database.Exec(`UPDATE users SET role='admin' WHERE id=1`); err != nil {
		t.Fatalf("promote admin: %v", err)
	}

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible, connection_info)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Preview Pentest", "desc", "pwn", 100, "", "pentest", "exact", `{}`, "always_on", "no_deploy", `{}`, true, `{"host":"10.10.1.50","port":"22"}`,
	)
	if err != nil {
		t.Fatalf("insert pentest challenge: %v", err)
	}
	instanceID, err := database.InsertGetID(
		`INSERT INTO instances (challenge_id, user_id, team_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag, reserved_cpu_mil, reserved_memory_mb, expires_at, status)
		 VALUES (?, NULL, NULL, ?, ?, ?, ?, ?, '', 0, 0, NULL, 'running')`,
		challengeID, "always_on", "no_deploy", "manual", "inst-old", `{"host":"10.10.1.50","port":"22"}`,
	)
	if err != nil {
		t.Fatalf("insert running instance: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/challenges/"+strconvFormat(challengeID)+"/workflow/restart", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("admin restart status = %d body=%s", rec.Code, rec.Body.String())
	}

	var oldStatus string
	if err := database.QueryRow(`SELECT status FROM instances WHERE id=?`, instanceID).Scan(&oldStatus); err != nil {
		t.Fatalf("query old instance: %v", err)
	}
	if oldStatus != "stopped" {
		t.Fatalf("old instance status = %q", oldStatus)
	}

	var count int
	if err := database.QueryRow(`SELECT COUNT(*) FROM instances WHERE challenge_id=? AND status='running'`, challengeID).Scan(&count); err != nil {
		t.Fatalf("count running instances: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly one running replacement instance, got %d", count)
	}

	var eventCount int
	if err := database.QueryRow(`SELECT COUNT(*) FROM challenge_restart_events WHERE challenge_id=? AND trigger_mode='admin' AND result='success'`, challengeID).Scan(&eventCount); err != nil {
		t.Fatalf("count restart events: %v", err)
	}
	if eventCount != 1 {
		t.Fatalf("expected one admin restart event, got %d", eventCount)
	}

	req = httptest.NewRequest(http.MethodGet, "/api/challenges/"+strconvFormat(challengeID)+"/workflow", nil)
	req.AddCookie(cookie)
	rec = httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("workflow status = %d body=%s", rec.Code, rec.Body.String())
	}

	var workflow map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &workflow); err != nil {
		t.Fatalf("decode workflow: %v", err)
	}
	if _, ok := workflow["service_instance"].(map[string]any); !ok {
		t.Fatalf("service_instance missing from workflow: %+v", workflow)
	}
	events, ok := workflow["restart_events"].([]any)
	if !ok || len(events) != 1 {
		t.Fatalf("restart_events missing from workflow: %+v", workflow)
	}
}

func TestExtendPerInstanceAddsConfiguredTTL(t *testing.T) {
	srv, database := newTestServer(t)
	srv.cfg.Deployer.InstanceTTL = "4h"
	cookie := registerAndCookie(t, srv)

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Extend Me", "desc", "web", 100, "", "dynamic_deploy", "exact", `{}`, "per_instance", "no_deploy", `{}`, true,
	)
	if err != nil {
		t.Fatalf("insert challenge: %v", err)
	}
	expiresAt := time.Now().Add(30 * time.Minute).UTC().Truncate(time.Second)
	if _, err := database.Exec(
		`INSERT INTO instances (challenge_id, user_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag, expires_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		challengeID, 1, "per_instance", "no_deploy", "manual", "inst-extend", `{"ip":"127.0.0.1","port":8080}`, "FLAG{x}", expiresAt, "running",
	); err != nil {
		t.Fatalf("insert instance: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/challenges/"+strconvFormat(challengeID)+"/instance/extend", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("extend status = %d body=%s", rec.Code, rec.Body.String())
	}

	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode extend response: %v", err)
	}
	rawExpires, _ := payload["expires_at"].(string)
	extended, err := time.Parse(time.RFC3339Nano, rawExpires)
	if err != nil {
		t.Fatalf("parse extended expires_at %q: %v", rawExpires, err)
	}
	wantMin := expiresAt.Add(4*time.Hour - time.Second)
	if extended.Before(wantMin) {
		t.Fatalf("expected expires_at to be extended from existing expiration, got %s want after %s", extended, wantMin)
	}
}

func TestCleanupExpiredInstancesStopsExpiredRows(t *testing.T) {
	srv, database := newTestServer(t)

	challengeID, err := database.InsertGetID(
		`INSERT INTO challenges (name, description, category, points, flag, challenge_type, flag_type, checker_config, deploy_type, deploy_backend, deploy_config, is_visible)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		"Expired", "desc", "web", 100, "", "dynamic_deploy", "exact", `{}`, "per_instance", "no_deploy", `{}`, true,
	)
	if err != nil {
		t.Fatalf("insert challenge: %v", err)
	}
	if _, err := database.Exec(
		`INSERT INTO instances (challenge_id, user_id, instance_type, backend, target_id, instance_id, connection_info, dynamic_flag, expires_at, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		challengeID, 1, "per_instance", "no_deploy", "manual", "expired-1", `{}`, "FLAG{x}", time.Now().Add(-time.Minute), "running",
	); err != nil {
		t.Fatalf("insert expired instance: %v", err)
	}

	stopped, err := srv.CleanupExpiredInstances(context.Background())
	if err != nil {
		t.Fatalf("cleanup expired instances: %v", err)
	}
	if stopped != 1 {
		t.Fatalf("expected one stopped instance, got %d", stopped)
	}
	var status string
	if err := database.QueryRow(`SELECT status FROM instances WHERE instance_id=?`, "expired-1").Scan(&status); err != nil {
		t.Fatalf("query expired instance status: %v", err)
	}
	if status != "stopped" {
		t.Fatalf("expected stopped status, got %q", status)
	}
}

func TestADGetVPNReturnsWireGuardConfig(t *testing.T) {
	srv, database := newTestServer(t)
	srv.cfg.CTF.TeamMode = true
	srv.cfg.AD.VPN.Enabled = true
	srv.cfg.AD.VPN.ServerPublicKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	srv.cfg.AD.VPN.ServerEndpoint = "vpn.example.com:51820"
	srv.cfg.AD.VPN.ServerIP = "10.8.0.1"
	srv.cfg.AD.VPN.TeamSubnetBase = "10.8."
	srv.cfg.AD.VPN.GameNetCIDR = "10.10.0.0/16"
	srv.cfg.AD.VPN.DNS = "1.1.1.1"
	srv.cfg.AD.VPN.HookCommand, srv.cfg.AD.VPN.HookArgs = vpnHookSuccessCommand()

	cookie := registerAndCookie(t, srv)
	attachUserToTeam(t, database, 1, "Red Team")

	req := httptest.NewRequest(http.MethodGet, "/api/ad/vpn", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("vpn status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if !strings.Contains(body, "Endpoint = vpn.example.com:51820") {
		t.Fatalf("unexpected vpn config body: %s", body)
	}
	if !strings.Contains(body, "AllowedIPs = 10.10.0.0/16, 10.8.0.1/32") {
		t.Fatalf("unexpected allowed ips: %s", body)
	}
}

func TestADSyncVPNRetriesProvisioning(t *testing.T) {
	srv, database := newTestServer(t)
	srv.cfg.CTF.TeamMode = true
	srv.cfg.AD.VPN.Enabled = true
	srv.cfg.AD.VPN.ServerPublicKey = "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
	srv.cfg.AD.VPN.ServerEndpoint = "vpn.example.com:51820"
	srv.cfg.AD.VPN.ServerIP = "10.8.0.1"
	srv.cfg.AD.VPN.TeamSubnetBase = "10.8."
	srv.cfg.AD.VPN.GameNetCIDR = "10.10.0.0/16"
	srv.cfg.AD.VPN.DNS = "1.1.1.1"
	srv.cfg.AD.VPN.HookCommand, srv.cfg.AD.VPN.HookArgs = vpnHookSuccessCommand()

	cookie := registerAndCookie(t, srv)
	teamID := attachUserToTeam(t, database, 1, "Green Team")
	if _, err := database.Exec(`INSERT INTO ad_vpn_peers (team_id, private_key, public_key, allowed_ip, provisioned, last_sync_error) VALUES (?, ?, ?, ?, ?, ?)`,
		teamID, "priv", "pub", "10.8.2.0/24", false, "previous failure"); err != nil {
		t.Fatalf("seed vpn peer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/ad/vpn/sync", nil)
	req.AddCookie(cookie)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("vpn sync status = %d body=%s", rec.Code, rec.Body.String())
	}

	var provisioned bool
	var lastErr string
	if err := database.QueryRow(`SELECT provisioned, last_sync_error FROM ad_vpn_peers WHERE team_id=?`, teamID).Scan(&provisioned, &lastErr); err != nil {
		t.Fatalf("query vpn peer: %v", err)
	}
	if !provisioned || lastErr != "" {
		t.Fatalf("unexpected vpn peer state: provisioned=%v last_error=%q", provisioned, lastErr)
	}
}

func strconvQuote(s string) string {
	b, _ := json.Marshal(s)
	return string(b)
}

func strconvFormat(n int64) string {
	b, _ := json.Marshal(n)
	return string(b)
}
