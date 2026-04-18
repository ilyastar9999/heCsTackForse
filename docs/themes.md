# Theme System

heCsTackForse uses a **CSS-variable-based theme system** that stores the active
theme as a JSON object in the `ctf_settings` table (key = `theme`) and delivers
it to the browser via `GET /api/theme`.

---

## How It Works

1. On every page load, the frontend calls `GET /api/theme`.
2. The response is a flat JSON object mapping CSS variable names → values:
   ```json
   {
     "--ctf-bg": "#0d0f13",
     "--ctf-card": "#161b22",
     "--ctf-accent": "#7e3ff2"
   }
   ```
3. A `<style>:root { … }</style>` block is injected before the first paint,
   overriding the defaults in `style.css`.

---

## Default Theme Variables

| Variable          | Default           | Description             |
|-------------------|-------------------|-------------------------|
| `--ctf-bg`        | `#0d0f13`         | Page background         |
| `--ctf-card`      | `#161b22`         | Card / panel background |
| `--ctf-text`      | `#e6edf3`         | Primary text            |
| `--ctf-muted`     | `#8b949e`         | Secondary / muted text  |
| `--ctf-accent`    | `#7e3ff2`         | Primary accent colour   |
| `--ctf-accent2`   | `#9d5bf4`         | Hover accent            |
| `--ctf-border`    | `#30363d`         | Border colour           |
| `--ctf-green`     | `#3fb950`         | Success / solved badge  |
| `--ctf-red`       | `#f85149`         | Error / danger          |
| `--ctf-yellow`    | `#d29922`         | Warning                 |
| `--ctf-blue`      | `#58a6ff`         | Info / links            |
| `--ctf-radius`    | `10px`            | Default border-radius   |

---

## Changing the Theme (Admin UI)

1. Go to **Admin → Config** tab.
2. Find the **Theme** section.
3. Enter a JSON object with the variables you want to override (you don't need
   to specify all of them — only overrides are needed).
4. Click **Save Theme**.

Example — orange accent:
```json
{
  "--ctf-accent": "#f97316",
  "--ctf-accent2": "#fb923c"
}
```

---

## Changing the Theme (API)

```http
PUT /api/admin/theme
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "vars": {
    "--ctf-accent": "#f97316",
    "--ctf-accent2": "#fb923c"
  }
}
```

---

## Resetting to Default

Send an empty object:
```http
PUT /api/admin/theme
{ "vars": {} }
```

Or delete the `theme` key from `ctf_settings` directly in the database.

---

## Built-in Presets

| Name        | Accent       | Background  |
|-------------|--------------|-------------|
| Dark Purple | `#7e3ff2`    | `#0d0f13`   |
| Cyber Green | `#00ff88`    | `#0a0a0a`   |
| Ocean Blue  | `#0ea5e9`    | `#0c1628`   |
| Hacker Red  | `#ef4444`    | `#0d0000`   |

Apply a preset from the Admin → Config → Theme tab.
