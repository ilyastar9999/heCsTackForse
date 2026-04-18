# Plugin System

heCsTackForse ships with an extensible plugin system modelled after CTFd's approach.
Plugins are pure Go packages that implement one of the defined interfaces and
self-register into the process-wide `plugin.Default` registry during `init()`.

---

## Plugin Interfaces

### `Plugin` (base interface)

Every plugin must implement:

```go
type Plugin interface {
    // Name returns the unique registry key for this plugin.
    Name() string
    // Init is called once after registration with any config from config.yaml.
    Init(cfg map[string]any) error
}
```

---

### `FlagChecker`

Controls how a submitted flag is validated against the stored correct flag.

```go
type FlagChecker interface {
    Plugin
    // Check returns true when submitted satisfies correctFlag.
    Check(correctFlag, submitted string) bool
}
```

#### Built-in flag checkers

| Name               | Behaviour                                              |
|--------------------|--------------------------------------------------------|
| `exact`            | Byte-perfect string comparison (default)               |
| `regex`            | `correctFlag` is a Go regexp; match with `MatchString` |
| `case_insensitive` | Unicode-aware case-insensitive exact match             |
| `prefix`           | Accepts any submission that starts with `correctFlag`  |

> **Backward compatibility:** `static` is automatically remapped to `exact`.

#### Per-challenge configuration

Set **Flag Type / Checker** in the Admin → Challenges form (or set `flag_type`
via the REST API) to any registered checker name.

---

### `Scorer`

Calculates the points awarded when a player solves a challenge.

```go
type Scorer interface {
    Plugin
    CalculateScore(challenge *models.Challenge, solveCount int) int
}
```

#### Built-in scorers

| Name      | Behaviour                                                             |
|-----------|-----------------------------------------------------------------------|
| `static`  | Always returns `challenge.Points` regardless of solveCount            |
| `dynamic` | Decays from `challenge.Points` toward `MinPoints` as solveCount grows |

The active scorer is selected via `ctf.scoring` in `config.yaml`:

```yaml
ctf:
  scoring: dynamic   # "static" | "dynamic" | <custom name>
```

---

### `Notifier`

Dispatches events (e.g. "flag solved", "new user") to external systems.

```go
type Notifier interface {
    Plugin
    Notify(event plugin.Event) error
}
```

---

## Writing a Custom Plugin

### 1. Create a Go package

```go
// myplugin/myplugin.go
package myplugin

import "github.com/ilyastar9999/heCsTackForse/internal/plugin"

type MyChecker struct{ secret string }

func (m *MyChecker) Name() string { return "my_checker" }

func (m *MyChecker) Init(cfg map[string]any) error {
    if s, ok := cfg["secret"].(string); ok {
        m.secret = s
    }
    return nil
}

func (m *MyChecker) Check(correct, submitted string) bool {
    // custom logic …
    return correct == submitted+m.secret
}

func init() {
    plugin.Default.RegisterFlagChecker(&MyChecker{})
}
```

### 2. Import the package (blank import) in `cmd/server/main.go`

```go
import (
    _ "github.com/ilyastar9999/heCsTackForse/myplugin"
)
```

### 3. Configure in `config.yaml` (optional)

```yaml
# No dedicated plugins section is required yet; configuration is passed through
# Init() when you add a Plugins list to the config in the future.
```

### 4. Use the checker in a challenge

Set `flag_type: my_checker` when creating or editing the challenge via the
Admin panel or `PUT /api/admin/challenges/:id`.

---

## Registry API

```go
// Register
plugin.Default.RegisterFlagChecker(fc plugin.FlagChecker)
plugin.Default.RegisterScorer(s plugin.Scorer)
plugin.Default.RegisterNotifier(n plugin.Notifier)

// Retrieve
checker, err := plugin.Default.GetFlagChecker("my_checker")
scorer,  err := plugin.Default.GetScorer("dynamic")

// List names
plugin.Default.FlagCheckerNames() // []string
plugin.Default.ScorerNames()      // []string
plugin.Default.NotifierNames()    // []string
```

All methods are safe for concurrent use.

---

## REST Endpoint

`GET /api/plugins` returns the names of all currently registered plugins:

```json
{
  "scorers":       ["static", "dynamic"],
  "flag_checkers": ["exact", "regex", "case_insensitive", "prefix"],
  "notifiers":     []
}
```

No authentication is required for this endpoint.
