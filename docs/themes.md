# Theme System

heCsTackForse supports a CTFd-style theme object with:

- theme bundle name
- CSS variable overrides
- custom CSS
- custom JS storage (kept for compatibility, not executed)

Theme settings are stored in `ctf_settings` with `key = theme` and returned by
`GET /api/theme`.

## How It Works

1. On each page load, the frontend calls `GET /api/theme`.
2. The response is normalized into:

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

3. The frontend applies, in order:

- theme bundle CSS from `/static/themes/{name}/theme.css` when `name != default`
- `vars` into `:root`
- `custom_css`

`custom_js` is intentionally not executed. Persisted theme JavaScript would turn
theme edits into stored XSS.

Legacy payloads that contain only CSS variables are still accepted and are
normalized to:

```json
{ "name": "default", "vars": { "...": "..." } }
```

## Default Theme Variables

| Variable | Default | Description |
| --- | --- | --- |
| `--ctf-bg` | `#0d0f13` | Page background |
| `--ctf-card` | `#161b22` | Card or panel background |
| `--ctf-text` | `#e6edf3` | Primary text |
| `--ctf-muted` | `#8b949e` | Secondary text |
| `--ctf-accent` | `#7e3ff2` | Primary accent |
| `--ctf-accent2` | `#9d5bf4` | Hover accent |
| `--ctf-border` | `#30363d` | Border color |
| `--ctf-green` | `#3fb950` | Success color |
| `--ctf-red` | `#f85149` | Error color |
| `--ctf-yellow` | `#d29922` | Warning color |
| `--ctf-blue` | `#58a6ff` | Info color |
| `--ctf-radius` | `10px` | Default border radius |

## Changing The Theme From Admin

1. Open `Admin -> Config -> Theme`.
2. Edit the JSON payload in the theme field.
3. Save it through the admin panel.

Example:

```json
{
  "name": "CTFD-crimson-theme",
  "vars": {},
  "custom_css": "",
  "custom_js": ""
}
```

You can also layer local overrides on top of an upstream theme:

```json
{
  "name": "CTFD-crimson-theme",
  "vars": {
    "--ctf-accent": "#f97316"
  },
  "custom_css": ".navbar { backdrop-filter: blur(8px); }",
  "custom_js": ""
}
```

## Changing The Theme By API

```http
PUT /api/admin/theme
Authorization: Bearer <admin_token>
Content-Type: application/json

{
  "name": "CTFD-crimson-theme",
  "vars": {},
  "custom_css": "",
  "custom_js": ""
}
```

## Resetting To Default

Send:

```http
PUT /api/admin/theme
{ "name": "default", "vars": {} }
```

Or delete the `theme` key from `ctf_settings`.

## Upstream Theme Mirror

The original `CTFd/themes` repositories are mirrored inside this project at:

- `themes/`
- `themes/ctfd-theme-tsgctf-dist/` for the built TSGCTF distribution

The application serves that mirror at:

- `/ctfd-themes/*`

The runtime theme entrypoints live at:

- `frontend/static/themes/{name}/theme.css`

These wrapper files import the original upstream CSS and add only a small
compatibility layer for heCsTackForse-specific markup.

## Available Theme Names

- `CTFD-crimson-theme`
- `CTFD-odin-theme`
- `CTFd-Dark-Theme`
- `CTFd-xmas-theme`
- `UnitedStates`
- `ctfd-neon-theme`
- `ftheme`
- `nullify-ctfd-theme`
- `pixo`
- `r2-8bit-theme`
- `r2-terminal`
- `r2-witchcraft`
- `watchdogs`

## TSGCTF Note

The upstream `ctfd-theme-tsgctf` theme is mirrored too, including its built
distribution, but it is implemented as a Nuxt application rather than a plain
CSS theme. The current heCsTackForse runtime loader is CSS-only, so `tsgctf` is
preserved in the repository mirror but not wired into `/static/themes/{name}`.
