# Plugin System

heCsTackForse ships with an extensible plugin system modelled after CTFd's approach.
Plugins are pure Go packages that implement one of the defined interfaces and
self-register into the process-wide `plugin.Default` registry during `init()`.

There is also an optional Python bridge for teams that want to reuse Python
plugin logic without rewriting it in Go. The bridge is not a full CTFd runtime;
it is a lightweight adapter that loads Python modules and maps them onto the
same Go plugin interfaces.

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

If `ctf.scoring` is set to any other registered scorer name, the server will
look it up in the registry and use it directly. That means Python bridge
scorers can be selected the same way as Go-native ones.

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

## Python Bridge

If you need to reuse Python code, enable the bridge in `config.yaml`:

```yaml
plugins:
  enabled: true
  python: python
  script: ./python/ctfd_bridge.py
  plugin_dirs:
    - ./python/ctfd_plugins
```

Each plugin module can expose a CTFd-style `load(app)` function, a
`register(registry)` function, or use the decorators exported by
`python/ctfd_bridge.py`.

The bridge provides a small CTFd compatibility shim for common imports such as
`CTFd.plugins.register_plugin_assets_directory`, menu registration helpers, and
decorators. It does not run Flask; plugins that depend on Flask request handlers
or SQLAlchemy internals need a small adapter that registers a heCsTackForse
notifier, scorer, flag checker, challenge type, or home widget.

```python
from ctfd_bridge import flag_checker, scorer

@flag_checker("my_checker")
class MyChecker:
    def name(self):
        return "my_checker"

    def check(self, correct_flag, submitted):
        return submitted == correct_flag


@scorer("my_score")
class MyScore:
    def name(self):
        return "my_score"

    def calculate_score(self, challenge, solve_count):
        return challenge.points
```

The bridge currently supports flag checkers, scorers, notifiers, and challenge
type extensions. It also supports home widgets rendered by the landing page.
Flag checkers, scorers, notifiers, and home widgets appear in `GET /api/plugins`.
Challenge types appear in `GET /api/challenge-types`. Flag checkers show up in
the challenge editor automatically.

### Example: CTFd Chat Notifier

`python/ctfd_plugins/CTFd_chat_notifier.py` demonstrates a CTFd-style
`load(app)` entrypoint:

```python
def load(app):
    app.register_notifier(ChatNotifier())
    app.register_home_widget(
        "chat_notifier",
        "Chat Notifier",
        "Solve and announcement notifications can be sent to Slack, Discord, or Telegram.",
        "/admin/plugins/chat-notifier",
    )
```

When the bridge is enabled, the plugin registers the `chat_notifier` notifier.
Admin settings under `/admin/plugins/chat-notifier` configure Slack, Discord, or
Telegram credentials. The landing page renders the plugin home widget when it is
loaded.

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

The challenge editor uses the same registry data to populate the flag checker
select, so externally loaded plugins are selectable without a code change.
