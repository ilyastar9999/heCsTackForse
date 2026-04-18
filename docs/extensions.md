# Extension System

heCsTackForse supports a CTFd-style extension system. Extensions are Go packages
that implement one or more plugin interfaces and self-register via `init()`.

---

## Challenge Type Extensions

A **ChallengeTypeExtension** lets you define entirely new challenge types — for
example "quiz", "king-of-the-hill", or a hardware challenge with a custom
verification flow.

### Interface

```go
type ChallengeTypeExtension interface {
    Plugin
    // TypeID is the unique string shown in the admin dropdown (e.g. "quiz").
    TypeID() string
    // Verify returns true when submitted satisfies the challenge.
    // challengeData is the raw JSON string stored in the challenge's deploy_config.
    Verify(challengeData, submitted string) bool
}
```

### Example: Quiz Challenge

```go
// quiz/quiz.go
package quiz

import (
    "encoding/json"

    "github.com/ilyastar9999/heCsTackForse/internal/plugin"
)

type QuizExtension struct{}

func (q *QuizExtension) Name() string    { return "quiz" }
func (q *QuizExtension) TypeID() string  { return "quiz" }
func (q *QuizExtension) Init(_ map[string]any) error { return nil }

func (q *QuizExtension) Verify(challengeData, submitted string) bool {
    var data struct {
        Answers []string `json:"answers"`
    }
    if err := json.Unmarshal([]byte(challengeData), &data); err != nil {
        return false
    }
    for _, a := range data.Answers {
        if a == submitted {
            return true
        }
    }
    return false
}

func init() {
    plugin.Default.RegisterChallengeType(&QuizExtension{})
}
```

### Registration

Import the extension package (blank import) in `cmd/server/main.go`:

```go
import (
    _ "github.com/ilyastar9999/heCsTackForse/quiz"
)
```

### Admin UI

Once registered, the type appears in the **Flag Type / Checker** dropdown in the
Admin → Challenges modal. Store any extra config in the **Deploy Config (JSON)**
field.

---

## Flag Checker Extensions

See `docs/plugins.md` for the `FlagChecker` interface. Any registered flag
checker also appears as a selectable challenge type in the Admin UI.

---

## Notifier Extensions

Implement `plugin.Notifier` to receive events:

```go
type MySlackNotifier struct{ webhook string }
func (n *MySlackNotifier) Name() string               { return "slack" }
func (n *MySlackNotifier) Init(cfg map[string]any) error {
    n.webhook, _ = cfg["webhook"].(string)
    return nil
}
func (n *MySlackNotifier) Notify(ev plugin.Event) error {
    // POST to Slack webhook …
    return nil
}
func init() { plugin.Default.RegisterNotifier(&MySlackNotifier{}) }
```

Events currently emitted:
| Event Type      | Data fields                          |
|-----------------|--------------------------------------|
| `flag_correct`  | `user_id`, `challenge_id`, `points`  |
| `user_register` | `user_id`, `username`                |

---

## REST Discovery

`GET /api/challenge-types` lists all registered challenge type IDs:

```json
["quiz", "hardware"]
```

`GET /api/plugins` lists all registered plugin names across all categories.

---

## Theme Extensions

Themes are **not** Go packages — they are CSS variable sets stored in the
database. See `docs/themes.md` for details.
