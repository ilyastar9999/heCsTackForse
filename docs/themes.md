# Theme System

heCsTackForse now supports a **full theme object** (CTFd-style) with:
- theme preset/bundle name,
- CSS variable overrides,
- custom CSS,
- custom JS.

Theme settings are stored in `ctf_settings` (`key = theme`) and delivered by
`GET /api/theme`.

---

## How It Works

1. On every page load, the frontend calls `GET /api/theme`.
2. The response is a theme JSON object:
   ```json
   {
     "name": "default",
     "vars": {
       "--ctf-bg": "#0d0f13",
       "--ctf-card": "#161b22",
       "--ctf-accent": "#7e3ff2"
     },
     "custom_css": "",
     "custom_js": ""
   }
   ```
3. Frontend applies, in order:
   - theme bundle CSS from `/static/themes/{name}/theme.css` (if `name != default`),
   - `vars` into `:root`,
   - `custom_css`,
   - `custom_js`.

Legacy support: old payloads with only CSS variables are still accepted and
automatically normalized to `{ "name": "default", "vars": { ... } }`.

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

Example — full theme payload:
```json
{
  "name": "default",
  "vars": {
    "--ctf-accent": "#f97316",
    "--ctf-accent2": "#fb923c"
  },
  "custom_css": ".navbar { backdrop-filter: blur(8px); }",
  "custom_js": "console.log('theme loaded')"
}
```

---

## Changing the Theme (API)

```http
PUT /api/admin/theme
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "default",
  "vars": {
    "--ctf-accent": "#f97316",
    "--ctf-accent2": "#fb923c"
  },
  "custom_css": "",
  "custom_js": ""
}
```

---

## Resetting to Default

Send an empty theme object:
```http
PUT /api/admin/theme
{ "name": "default", "vars": {} }
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
