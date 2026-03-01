// Package ad implements the Attack & Defence round engine.
//
// Each round the engine:
//  1. Records the round start in ad_rounds.
//  2. Rotates flags for every (challenge, team) pair that has a running instance.
//  3. Runs admin-configured checker scripts against every team's service to
//     determine service availability and award defence points.
//  4. Runs all enabled team sploits against every OTHER team's service instance;
//     any flags extracted from stdout are submitted to the central flag submitter
//     (configured via ad.flag_submit_url).
//  5. Closes the round record.
package ad

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/ilyastar9999/heCsTackForse/internal/config"
	"github.com/ilyastar9999/heCsTackForse/internal/db"
)

// flagRE matches the default FLAG{…} format; the engine also tries to detect
// any word that looks like a flag from sploit stdout.
var flagRE = regexp.MustCompile(`FLAG\{[^}]+\}`)

// Engine drives the A&D game loop.
type Engine struct {
	db           *db.DB
	cfg          *config.Config
	roundDur     time.Duration
	currentRound atomic.Int64
	stopCh       chan struct{}
	wg           sync.WaitGroup
	httpClient   *http.Client
}

// New creates an Engine but does not start it.
func New(database *db.DB, cfg *config.Config) (*Engine, error) {
	roundDur, err := time.ParseDuration(cfg.AD.RoundDuration)
	if err != nil {
		return nil, fmt.Errorf("ad: invalid round_duration %q: %w", cfg.AD.RoundDuration, err)
	}
	return &Engine{
		db:         database,
		cfg:        cfg,
		roundDur:   roundDur,
		stopCh:     make(chan struct{}),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Start launches the round ticker in a background goroutine.
func (e *Engine) Start() {
	e.wg.Add(1)
	go func() {
		defer e.wg.Done()
		ticker := time.NewTicker(e.roundDur)
		defer ticker.Stop()
		log.Printf("ad engine: started (round=%s)", e.roundDur)
		for {
			select {
			case <-ticker.C:
				round := e.currentRound.Add(1)
				e.runRound(round)
			case <-e.stopCh:
				log.Println("ad engine: stopped")
				return
			}
		}
	}()
}

// Stop signals the engine to shut down and waits for the current round to finish.
func (e *Engine) Stop() {
	close(e.stopCh)
	e.wg.Wait()
}

// CurrentRound returns the last completed round number.
func (e *Engine) CurrentRound() int64 {
	return e.currentRound.Load()
}

// ──────────────────────────────────────────────────────────────────────────────
// Round execution
// ──────────────────────────────────────────────────────────────────────────────

func (e *Engine) runRound(round int64) {
	log.Printf("ad engine: round %d starting", round)
	start := time.Now()

	// Record round start
	_, _ = e.db.Exec(
		`INSERT OR IGNORE INTO ad_rounds (round, started_at) VALUES (?, ?)`,
		round, start,
	)

	e.rotateFlags(round)
	e.runCheckers(round)
	e.runSploits(round)

	_, _ = e.db.Exec(
		`UPDATE ad_rounds SET finished_at=? WHERE round=?`,
		time.Now(), round,
	)
	log.Printf("ad engine: round %d finished in %s", round, time.Since(start).Round(time.Millisecond))
}

// ──────────────────────────────────────────────────────────────────────────────
// 1. Flag rotation
// ──────────────────────────────────────────────────────────────────────────────

// rotateFlags generates a fresh flag for every (challenge, instance) pair and
// stores it in ad_flags.  The actual flag injection into the running service
// (e.g. writing to a file inside the container) is left to the per-challenge
// checker/deploy script since the mechanism is service-specific.
func (e *Engine) rotateFlags(round int64) {
	rows, err := e.db.Query(`
		SELECT i.challenge_id, i.team_id, i.user_id
		FROM instances i
		WHERE i.status='running'
	`)
	if err != nil {
		log.Printf("ad: rotateFlags query error: %v", err)
		return
	}
	defer rows.Close()

	prefix := e.cfg.CTF.FlagPrefix
	for rows.Next() {
		var challengeID int64
		var teamID, userID sql.NullInt64
		if err := rows.Scan(&challengeID, &teamID, &userID); err != nil {
			continue
		}
		flag := generateFlag(prefix)
		_, _ = e.db.Exec(
			`INSERT INTO ad_flags (challenge_id, team_id, flag, round) VALUES (?, ?, ?, ?)`,
			challengeID, teamID.Int64, flag, round,
		)
	}
}

func generateFlag(prefix string) string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return prefix + hex.EncodeToString(b) + "}"
}

// ──────────────────────────────────────────────────────────────────────────────
// 2. Checker runner
// ──────────────────────────────────────────────────────────────────────────────

// checkerScriptForChallenge returns the checker_script field from deploy_config
// JSON for the given challenge id.  Admin sets this when creating the challenge.
func (e *Engine) checkerScriptForChallenge(challengeID int64) string {
	var deployConfig string
	_ = e.db.QueryRow(`SELECT deploy_config FROM challenges WHERE id=?`, challengeID).Scan(&deployConfig)
	var cfg map[string]any
	if err := json.Unmarshal([]byte(deployConfig), &cfg); err != nil {
		return ""
	}
	s, _ := cfg["checker_script"].(string)
	return s
}

// runCheckers calls the checker script for every running instance and records
// service availability in ad_services.
func (e *Engine) runCheckers(round int64) {
	timeout, _ := time.ParseDuration(e.cfg.AD.CheckerTimeout)
	if timeout <= 0 {
		timeout = 30 * time.Second
	}

	rows, err := e.db.Query(`
		SELECT i.id, i.challenge_id, i.team_id, i.connection_info
		FROM instances i
		WHERE i.status='running'
	`)
	if err != nil {
		log.Printf("ad: runCheckers query error: %v", err)
		return
	}
	defer rows.Close()

	for rows.Next() {
		var instanceID int64
		var challengeID int64
		var teamID sql.NullInt64
		var connInfoJSON string
		if err := rows.Scan(&instanceID, &challengeID, &teamID, &connInfoJSON); err != nil {
			continue
		}

		script := e.checkerScriptForChallenge(challengeID)
		if script == "" {
			// No checker configured — assume up.
			e.recordServiceStatus(challengeID, teamID.Int64, "up", round, 1)
			continue
		}

		var connInfo map[string]any
		_ = json.Unmarshal([]byte(connInfoJSON), &connInfo)
		host, _ := connInfo["host"].(string)
		port, _ := connInfo["port"].(string)

		status, score := e.execChecker(script, host, port, timeout)
		e.recordServiceStatus(challengeID, teamID.Int64, status, round, score)
	}
}

// execChecker runs a checker script and returns service status + score.
// The script receives HOST and PORT env vars; exit code 0 = up, 1 = down,
// 2 = corrupt.
func (e *Engine) execChecker(script, host, port string, timeout time.Duration) (status string, score int) {
	f, err := os.CreateTemp(e.cfg.AD.SploitDir, "checker-*.sh")
	if err != nil {
		return "down", 0
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(script); err != nil {
		return "down", 0
	}
	f.Close()
	if err := os.Chmod(f.Name(), 0o700); err != nil {
		return "down", 0
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, f.Name())
	cmd.Env = append(os.Environ(), "HOST="+host, "PORT="+port)
	if err := cmd.Run(); err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			return "down", 0
		}
		exitCode := 0
		if ee, ok := err.(*exec.ExitError); ok {
			exitCode = ee.ExitCode()
		}
		switch exitCode {
		case 2:
			return "corrupt", 0
		default:
			return "down", 0
		}
	}
	return "up", 1
}

func (e *Engine) recordServiceStatus(challengeID, teamID int64, status string, round int64, score int) {
	_, _ = e.db.Exec(
		`INSERT INTO ad_services (challenge_id, team_id, status, round, score, checked_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		challengeID, teamID, status, round, score, time.Now(),
	)
	// Award defence points to the team's score if service is up
	if score > 0 {
		_, _ = e.db.Exec(`UPDATE teams SET score=score+1 WHERE id=?`, teamID)
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 3. Sploit runner
// ──────────────────────────────────────────────────────────────────────────────

// runSploits fetches all enabled sploits and runs each one against every other
// team's service instance in parallel.
func (e *Engine) runSploits(round int64) {
	if err := os.MkdirAll(e.cfg.AD.SploitDir, 0o700); err != nil {
		log.Printf("ad: cannot create sploit dir %s: %v", e.cfg.AD.SploitDir, err)
	}

	// Fetch enabled sploits
	sploitRows, err := e.db.Query(`
		SELECT id, team_id, challenge_id, language, script
		FROM ad_sploits WHERE enabled=1
	`)
	if err != nil {
		log.Printf("ad: runSploits query error: %v", err)
		return
	}
	type sploitRec struct {
		id, teamID, challengeID int64
		language, script        string
	}
	var sploits []sploitRec
	for sploitRows.Next() {
		var s sploitRec
		if err := sploitRows.Scan(&s.id, &s.teamID, &s.challengeID, &s.language, &s.script); err != nil {
			continue
		}
		sploits = append(sploits, s)
	}
	sploitRows.Close()

	if len(sploits) == 0 {
		return
	}

	timeout, _ := time.ParseDuration(e.cfg.AD.SploitTimeout)
	if timeout <= 0 {
		timeout = 60 * time.Second
	}

	var mu sync.Mutex
	var capturedFlags []string

	// For each sploit, run against every other team's instance of the same challenge
	var wg sync.WaitGroup
	for _, sp := range sploits {
		sp := sp
		// Get all running instances for this challenge except the sploit-owner's team
		instRows, err := e.db.Query(`
			SELECT i.team_id, i.connection_info
			FROM instances i
			WHERE i.challenge_id=? AND i.status='running' AND (i.team_id IS NULL OR i.team_id != ?)
		`, sp.challengeID, sp.teamID)
		if err != nil {
			continue
		}
		type target struct {
			teamID     int64
			connInfo   map[string]any
		}
		var targets []target
		for instRows.Next() {
			var tid sql.NullInt64
			var connJSON string
			if err := instRows.Scan(&tid, &connJSON); err != nil {
				continue
			}
			var info map[string]any
			_ = json.Unmarshal([]byte(connJSON), &info)
			targets = append(targets, target{teamID: tid.Int64, connInfo: info})
		}
		instRows.Close()

		for _, t := range targets {
			t := t
			sp := sp
			wg.Add(1)
			go func() {
				defer wg.Done()
				host, _ := t.connInfo["host"].(string)
				port, _ := t.connInfo["port"].(string)
				stdout, flags, runErr := e.execSploit(sp.language, sp.script, host, port, timeout)
				errStr := ""
				if runErr != nil {
					errStr = runErr.Error()
				}
				// Record result
				_, _ = e.db.Exec(`
					INSERT INTO ad_sploit_results
						(sploit_id, target_team_id, round, stdout, flags_captured, error, ran_at)
					VALUES (?, ?, ?, ?, ?, ?, ?)`,
					sp.id, t.teamID, round, stdout, len(flags), errStr, time.Now(),
				)
				// Update sploit last_run_at
				_, _ = e.db.Exec(`UPDATE ad_sploits SET last_run_at=? WHERE id=?`, time.Now(), sp.id)

				if len(flags) > 0 {
					mu.Lock()
					capturedFlags = append(capturedFlags, flags...)
					mu.Unlock()
				}
			}()
		}
	}
	wg.Wait()

	if len(capturedFlags) > 0 {
		e.submitFlagsToServer(capturedFlags, round)
	}
}

// execSploit writes the script to a temp file, runs it with HOST/PORT env vars,
// and extracts flags from stdout.
func (e *Engine) execSploit(language, script, host, port string, timeout time.Duration) (stdout string, flags []string, err error) {
	ext := scriptExtension(language)
	f, ferr := os.CreateTemp(e.cfg.AD.SploitDir, "sploit-*"+ext)
	if ferr != nil {
		return "", nil, ferr
	}
	defer os.Remove(f.Name())
	if _, werr := f.WriteString(script); werr != nil {
		return "", nil, werr
	}
	f.Close()
	if err = os.Chmod(f.Name(), 0o700); err != nil {
		return "", nil, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var cmd *exec.Cmd
	switch language {
	case "python3", "python":
		cmd = exec.CommandContext(ctx, "python3", f.Name())
	case "bash", "sh":
		cmd = exec.CommandContext(ctx, "bash", f.Name())
	default:
		cmd = exec.CommandContext(ctx, f.Name())
	}
	absDir, _ := filepath.Abs(e.cfg.AD.SploitDir)
	cmd.Dir = absDir
	cmd.Env = append(os.Environ(), "HOST="+host, "PORT="+port, "TARGET="+host)

	var buf bytes.Buffer
	cmd.Stdout = io.MultiWriter(&buf, os.Stdout)
	cmd.Stderr = &buf

	if runErr := cmd.Run(); runErr != nil {
		if ctx.Err() != context.DeadlineExceeded {
			err = runErr
		}
	}
	stdout = buf.String()
	// Extract flags — every line matching FLAG{...} pattern
	prefix := e.cfg.CTF.FlagPrefix
	re := flagRE
	if prefix != "FLAG{" {
		escaped := regexp.QuoteMeta(strings.TrimSuffix(prefix, "{"))
		re = regexp.MustCompile(escaped + `\{[^}]+\}`)
	}
	flags = re.FindAllString(stdout, -1)
	return
}

func scriptExtension(lang string) string {
	switch lang {
	case "python3", "python":
		return ".py"
	case "bash", "sh":
		return ".sh"
	default:
		return ""
	}
}

// ──────────────────────────────────────────────────────────────────────────────
// 4. Central flag submitter
// ──────────────────────────────────────────────────────────────────────────────

// submitFlagsToServer POSTs captured flags to the central flag submission
// server (e.g. http://10.10.10.10/flags).  The server is expected to accept a
// JSON array of flag strings.  Many AD systems also support a simple
// newline-delimited text format — this implementation tries JSON first.
func (e *Engine) submitFlagsToServer(flags []string, round int64) {
	if e.cfg.AD.FlagSubmitURL == "" || len(flags) == 0 {
		return
	}
	// Deduplicate
	seen := make(map[string]struct{}, len(flags))
	uniq := flags[:0]
	for _, f := range flags {
		if _, ok := seen[f]; !ok {
			seen[f] = struct{}{}
			uniq = append(uniq, f)
		}
	}

	body, err := json.Marshal(uniq)
	if err != nil {
		log.Printf("ad: submitFlags marshal error: %v", err)
		return
	}

	req, err := http.NewRequest(http.MethodPost, e.cfg.AD.FlagSubmitURL, bytes.NewReader(body))
	if err != nil {
		log.Printf("ad: submitFlags build request error: %v", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	if e.cfg.AD.FlagSubmitKey != "" {
		req.Header.Set("Authorization", "Bearer "+e.cfg.AD.FlagSubmitKey)
		req.Header.Set("X-Team-Token", e.cfg.AD.FlagSubmitKey)
	}

	resp, err := e.httpClient.Do(req)
	if err != nil {
		log.Printf("ad: submitFlags HTTP error: %v", err)
		return
	}
	defer resp.Body.Close()

	// Update flags_submitted count in the most-recent results rows
	_, _ = e.db.Exec(`
		UPDATE ad_sploit_results
		SET flags_submitted = flags_captured
		WHERE round=? AND flags_submitted=0 AND flags_captured>0`,
		round,
	)
	log.Printf("ad: round %d — submitted %d flags to %s (HTTP %d)",
		round, len(uniq), e.cfg.AD.FlagSubmitURL, resp.StatusCode)
}
