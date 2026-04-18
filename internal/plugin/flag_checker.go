package plugin

import (
	"regexp"
	"strings"
)

// FlagChecker is a plugin that decides whether a submitted flag is correct.
// Implement this interface and register it via plugin.Default.RegisterFlagChecker()
// to add a custom flag verification strategy.
type FlagChecker interface {
	Plugin
	// Check returns true when submitted satisfies the stored correctFlag value.
	Check(correctFlag, submitted string) bool
}

// ExactChecker requires a byte-perfect string match (default).
type ExactChecker struct{}

func (e *ExactChecker) Name() string                { return "exact" }
func (e *ExactChecker) Init(_ map[string]any) error { return nil }
func (e *ExactChecker) Check(correct, submitted string) bool {
	return correct == submitted
}

// RegexChecker treats correctFlag as a Go regular-expression pattern.
// The submission is accepted when it matches the pattern.
type RegexChecker struct{}

func (r *RegexChecker) Name() string                { return "regex" }
func (r *RegexChecker) Init(_ map[string]any) error { return nil }
func (r *RegexChecker) Check(correct, submitted string) bool {
	re, err := regexp.Compile(correct)
	if err != nil {
		return false
	}
	return re.MatchString(submitted)
}

// CaseInsensitiveChecker performs a case-insensitive Unicode-aware exact match.
type CaseInsensitiveChecker struct{}

func (c *CaseInsensitiveChecker) Name() string                { return "case_insensitive" }
func (c *CaseInsensitiveChecker) Init(_ map[string]any) error { return nil }
func (c *CaseInsensitiveChecker) Check(correct, submitted string) bool {
	return strings.EqualFold(correct, submitted)
}

// PrefixChecker accepts any submission that starts with correctFlag.
// Useful for flag-prefix validation without a fixed suffix.
type PrefixChecker struct{}

func (p *PrefixChecker) Name() string                { return "prefix" }
func (p *PrefixChecker) Init(_ map[string]any) error { return nil }
func (p *PrefixChecker) Check(correct, submitted string) bool {
	return strings.HasPrefix(submitted, correct)
}
