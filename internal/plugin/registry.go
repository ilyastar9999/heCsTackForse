package plugin

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Registry holds all registered plugin implementations keyed by their Name().
// Plugins are safe for concurrent use after registration.
type Registry struct {
	mu             sync.RWMutex
	scorers        map[string]Scorer
	flagCheckers   map[string]FlagChecker
	notifiers      map[string]Notifier
	challengeTypes map[string]ChallengeTypeExtension
	homeWidgets    map[string]HomeWidget
	adminMenu      map[string]AdminMenuEntry
}

// Default is the process-wide plugin registry.
// Built-in plugins are pre-registered in the package init() function.
// Third-party plugins should call Default.Register* during their own init().
var Default = newRegistry()

func newRegistry() *Registry {
	return &Registry{
		scorers:        make(map[string]Scorer),
		flagCheckers:   make(map[string]FlagChecker),
		notifiers:      make(map[string]Notifier),
		challengeTypes: make(map[string]ChallengeTypeExtension),
		homeWidgets:    make(map[string]HomeWidget),
		adminMenu:      make(map[string]AdminMenuEntry),
	}
}

func init() {
	// Built-in scorers
	Default.RegisterScorer(&StaticScorer{})
	Default.RegisterScorer(&DynamicScorer{MinPoints: 50, Decay: 0.05})

	// Built-in flag checkers
	Default.RegisterFlagChecker(&ExactChecker{})
	Default.RegisterFlagChecker(&RegexChecker{})
	Default.RegisterFlagChecker(&CaseInsensitiveChecker{})
	Default.RegisterFlagChecker(&PrefixChecker{})

	// Built-in challenge types
	Default.RegisterChallengeType(&StaticChallengeType{})
	Default.RegisterChallengeType(&DynamicDeployChallengeType{})
	Default.RegisterChallengeType(&PentestChallengeType{})
	Default.RegisterChallengeType(&ADAttackChallengeType{})
	Default.RegisterChallengeType(&ADDefenseChallengeType{})
}

// ─── Scorer ──────────────────────────────────────────────────────────────────

// RegisterScorer adds or replaces a Scorer. Safe for concurrent use.
func (r *Registry) RegisterScorer(s Scorer) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.scorers[s.Name()] = s
}

// GetScorer retrieves a Scorer by name, returning an error if not found.
func (r *Registry) GetScorer(name string) (Scorer, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.scorers[name]
	if !ok {
		return nil, fmt.Errorf("scorer %q not registered", name)
	}
	return s, nil
}

// ScorerNames returns the names of all registered scorers.
func (r *Registry) ScorerNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.scorers))
	for n := range r.scorers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ─── FlagChecker ─────────────────────────────────────────────────────────────

// RegisterFlagChecker adds or replaces a FlagChecker. Safe for concurrent use.
func (r *Registry) RegisterFlagChecker(fc FlagChecker) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.flagCheckers[fc.Name()] = fc
}

// GetFlagChecker retrieves a FlagChecker by name.
// "static" is a legacy alias for "exact" and is transparently remapped.
func (r *Registry) GetFlagChecker(name string) (FlagChecker, error) {
	if name == "static" {
		name = "exact" // backward-compat alias
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	fc, ok := r.flagCheckers[name]
	if !ok {
		return nil, fmt.Errorf("flag checker %q not registered", name)
	}
	return fc, nil
}

// FlagCheckerNames returns the names of all registered flag checkers.
func (r *Registry) FlagCheckerNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.flagCheckers))
	for n := range r.flagCheckers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ─── Notifier ────────────────────────────────────────────────────────────────

// RegisterNotifier adds or replaces a Notifier. Safe for concurrent use.
func (r *Registry) RegisterNotifier(n Notifier) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.notifiers[n.Name()] = n
}

// GetNotifier retrieves a Notifier by name, returning an error if not found.
func (r *Registry) GetNotifier(name string) (Notifier, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	n, ok := r.notifiers[name]
	if !ok {
		return nil, fmt.Errorf("notifier %q not registered", name)
	}
	return n, nil
}

// NotifierNames returns the names of all registered notifiers.
func (r *Registry) NotifierNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.notifiers))
	for n := range r.notifiers {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

// ─── ChallengeTypeExtension ───────────────────────────────────────────────────

// RegisterChallengeType adds or replaces a ChallengeTypeExtension. Safe for concurrent use.
func (r *Registry) RegisterChallengeType(ext ChallengeTypeExtension) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.challengeTypes[ext.TypeID()] = ext
}

// GetChallengeType retrieves a ChallengeTypeExtension by name, returning an error if not found.
func (r *Registry) GetChallengeType(name string) (ChallengeTypeExtension, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ext, ok := r.challengeTypes[name]
	if !ok {
		return nil, fmt.Errorf("challenge type %q not registered", name)
	}
	return ext, nil
}

// ChallengeTypeNames returns the names of all registered challenge types.
func (r *Registry) ChallengeTypeNames() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.challengeTypes))
	for n := range r.challengeTypes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (r *Registry) ChallengeTypes() []ChallengeTypeSpec {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.challengeTypes))
	for n := range r.challengeTypes {
		names = append(names, n)
	}
	sort.Strings(names)

	specs := make([]ChallengeTypeSpec, 0, len(names))
	for _, name := range names {
		spec := ChallengeTypeSpec{
			ID:                     name,
			Title:                  humanizePluginName(name),
			SubmissionMode:         "plugin",
			AccessMode:             "plugin",
			SupportsManualFlags:    true,
			SupportsCheckerConfig:  true,
			SupportsFiles:          true,
			SupportsHints:          true,
			SupportsConnectionInfo: true,
		}
		if describer, ok := r.challengeTypes[name].(ChallengeTypeDescriber); ok {
			custom := describer.Descriptor()
			if custom.ID == "" {
				custom.ID = name
			}
			if custom.Title == "" {
				custom.Title = humanizePluginName(custom.ID)
			}
			spec = custom
		}
		specs = append(specs, spec)
	}
	return specs
}

func (r *Registry) RegisterHomeWidget(w HomeWidget) {
	if w.Name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.homeWidgets[w.Name] = w
}

func (r *Registry) HomeWidgets() []HomeWidget {
	r.mu.RLock()
	defer r.mu.RUnlock()
	widgets := make([]HomeWidget, 0, len(r.homeWidgets))
	for _, w := range r.homeWidgets {
		widgets = append(widgets, w)
	}
	sort.Slice(widgets, func(i, j int) bool { return widgets[i].Name < widgets[j].Name })
	return widgets
}

func (r *Registry) RegisterAdminMenu(entry AdminMenuEntry) {
	if entry.Name == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	r.adminMenu[entry.Name] = entry
}

func (r *Registry) AdminMenu() []AdminMenuEntry {
	r.mu.RLock()
	defer r.mu.RUnlock()
	menu := make([]AdminMenuEntry, 0, len(r.adminMenu))
	for _, item := range r.adminMenu {
		menu = append(menu, item)
	}
	sort.Slice(menu, func(i, j int) bool { return menu[i].Route < menu[j].Route })
	return menu
}

func humanizePluginName(name string) string {
	if name == "" {
		return ""
	}
	parts := strings.FieldsFunc(name, func(r rune) bool {
		return r == '_' || r == '-'
	})
	for i, part := range parts {
		if part == "" {
			continue
		}
		parts[i] = strings.ToUpper(part[:1]) + strings.ToLower(part[1:])
	}
	return strings.Join(parts, " ")
}
