package pluginbridge

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"sync"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
	"github.com/ilyastar9999/heCsTackForse/internal/plugin"
)

type Config struct {
	Enabled    bool     `yaml:"enabled"`
	Python     string   `yaml:"python"`
	Script     string   `yaml:"script"`
	PluginDirs []string `yaml:"plugin_dirs"`
	Modules    []string `yaml:"modules"`
}

type Catalog struct {
	FlagCheckers   []string `json:"flag_checkers"`
	Scorers        []string `json:"scorers"`
	Notifiers      []string `json:"notifiers"`
	ChallengeTypes []string `json:"challenge_types"`
	Assets         []any    `json:"assets"`
	AdminMenu      []any    `json:"admin_menu"`
	UserMenu       []any    `json:"user_menu"`
	HomeWidgets    []any    `json:"home_widgets"`
}

type Bridge struct {
	cmd     *exec.Cmd
	stdin   io.WriteCloser
	decoder *json.Decoder
	encoder *json.Encoder
	mu      sync.Mutex
	closed  bool
	catalog Catalog
	stop    sync.Once
}

type request struct {
	Op     string `json:"op"`
	Params any    `json:"params,omitempty"`
}

type response struct {
	OK     bool            `json:"ok"`
	Error  string          `json:"error,omitempty"`
	Result json.RawMessage `json:"result,omitempty"`
}

func New(cfg Config) (*Bridge, error) {
	python := cfg.Python
	if python == "" {
		python = "python"
	}
	script := cfg.Script
	if script == "" {
		script = "./python/ctfd_bridge.py"
	}

	args := []string{script}
	for _, dir := range cfg.PluginDirs {
		if dir != "" {
			args = append(args, "--plugin-dir", dir)
		}
	}
	for _, module := range cfg.Modules {
		if module != "" {
			args = append(args, "--module", module)
		}
	}

	cmd := exec.Command(python, args...)
	cmd.Stderr = os.Stderr
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		return nil, err
	}

	b := &Bridge{
		cmd:     cmd,
		stdin:   stdin,
		decoder: json.NewDecoder(bufio.NewReader(stdout)),
		encoder: json.NewEncoder(stdin),
	}
	if err := b.refreshCatalog(); err != nil {
		_ = b.Close()
		return nil, err
	}
	return b, nil
}

func (b *Bridge) refreshCatalog() error {
	var catalog Catalog
	if err := b.call("list", nil, &catalog); err != nil {
		return err
	}
	b.catalog = catalog
	return nil
}

func (b *Bridge) Catalog() Catalog {
	return b.catalog
}

func (b *Bridge) Close() error {
	var closeErr error
	b.stop.Do(func() {
		b.mu.Lock()
		b.closed = true
		if b.stdin != nil {
			_ = b.encoder.Encode(request{Op: "shutdown"})
			_ = b.stdin.Close()
		}
		b.mu.Unlock()
		if b.cmd != nil && b.cmd.Process != nil {
			if err := b.cmd.Wait(); err != nil && !errors.Is(err, exec.ErrNotFound) {
				closeErr = err
			}
		}
	})
	return closeErr
}

func (b *Bridge) call(op string, params any, out any) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return errors.New("plugin bridge is closed")
	}
	if err := b.encoder.Encode(request{Op: op, Params: params}); err != nil {
		return err
	}
	var resp response
	if err := b.decoder.Decode(&resp); err != nil {
		return err
	}
	if !resp.OK {
		if resp.Error == "" {
			return errors.New("plugin bridge request failed")
		}
		return errors.New(resp.Error)
	}
	if out == nil || len(resp.Result) == 0 {
		return nil
	}
	return json.Unmarshal(resp.Result, out)
}

func (b *Bridge) FlagChecker(name string) plugin.FlagChecker {
	return &flagCheckerAdapter{bridge: b, name: name}
}

func (b *Bridge) Scorer(name string) plugin.Scorer {
	return &scorerAdapter{bridge: b, name: name}
}

func (b *Bridge) Notifier(name string) plugin.Notifier {
	return &notifierAdapter{bridge: b, name: name}
}

func (b *Bridge) ChallengeType(name string) plugin.ChallengeTypeExtension {
	return &challengeTypeAdapter{bridge: b, name: name}
}

type flagCheckerAdapter struct {
	bridge *Bridge
	name   string
}

func (a *flagCheckerAdapter) Name() string                { return a.name }
func (a *flagCheckerAdapter) Init(_ map[string]any) error { return nil }
func (a *flagCheckerAdapter) Check(correctFlag, submitted string) bool {
	var result bool
	if err := a.bridge.call("check_flag", map[string]any{
		"name":         a.name,
		"correct_flag": correctFlag,
		"submitted":    submitted,
	}, &result); err != nil {
		return false
	}
	return result
}

type scorerAdapter struct {
	bridge *Bridge
	name   string
}

func (a *scorerAdapter) Name() string                { return a.name }
func (a *scorerAdapter) Init(_ map[string]any) error { return nil }
func (a *scorerAdapter) CalculateScore(challenge *models.Challenge, solveCount int) int {
	var result int
	if err := a.bridge.call("calculate_score", map[string]any{
		"name":        a.name,
		"challenge":   challenge,
		"solve_count": solveCount,
	}, &result); err != nil {
		return challenge.Points
	}
	return result
}

type notifierAdapter struct {
	bridge *Bridge
	name   string
}

func (a *notifierAdapter) Name() string                { return a.name }
func (a *notifierAdapter) Init(_ map[string]any) error { return nil }
func (a *notifierAdapter) Notify(event plugin.Event) error {
	return a.bridge.call("notify", map[string]any{
		"name":  a.name,
		"event": event,
	}, nil)
}

type challengeTypeAdapter struct {
	bridge *Bridge
	name   string
}

func (a *challengeTypeAdapter) Name() string                { return a.name }
func (a *challengeTypeAdapter) Init(_ map[string]any) error { return nil }
func (a *challengeTypeAdapter) TypeID() string              { return a.name }
func (a *challengeTypeAdapter) Descriptor() plugin.ChallengeTypeSpec {
	return plugin.ChallengeTypeSpec{
		ID:                     a.name,
		Title:                  a.name,
		Description:            "Plugin-provided challenge type.",
		SubmissionMode:         "plugin",
		AccessMode:             "plugin",
		SupportsManualFlags:    true,
		SupportsCheckerConfig:  true,
		SupportsFiles:          true,
		SupportsHints:          true,
		SupportsConnectionInfo: true,
	}
}
func (a *challengeTypeAdapter) Verify(challengeData, submitted string) bool {
	var result bool
	if err := a.bridge.call("verify_challenge_type", map[string]any{
		"name":           a.name,
		"challenge_data": challengeData,
		"submitted":      submitted,
	}, &result); err != nil {
		return false
	}
	return result
}
