package plugin

import (
	"fmt"
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
	return names
}
