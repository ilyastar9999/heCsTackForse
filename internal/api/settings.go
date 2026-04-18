package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"
)

type themeConfig struct {
	Name      string            `json:"name,omitempty"`
	Vars      map[string]string `json:"vars,omitempty"`
	CustomCSS string            `json:"custom_css,omitempty"`
	CustomJS  string            `json:"custom_js,omitempty"`
}

func normalizeThemePayload(raw map[string]any) themeConfig {
	cfg := themeConfig{Name: "default", Vars: map[string]string{}}
	if raw == nil {
		return cfg
	}

	_, hasVars := raw["vars"]
	_, hasName := raw["name"]
	_, hasCustomCSS := raw["custom_css"]
	_, hasCustomJS := raw["custom_js"]

	if !hasVars && !hasName && !hasCustomCSS && !hasCustomJS {
		for k, v := range raw {
			cfg.Vars[k] = toString(v)
		}
		return cfg
	}

	if name, ok := raw["name"].(string); ok && name != "" {
		cfg.Name = name
	}
	if css, ok := raw["custom_css"].(string); ok {
		cfg.CustomCSS = css
	}
	if js, ok := raw["custom_js"].(string); ok {
		cfg.CustomJS = js
	}

	if varsRaw, ok := raw["vars"].(map[string]any); ok {
		for k, v := range varsRaw {
			cfg.Vars[k] = toString(v)
		}
	}

	return cfg
}

func toString(v any) string {
	if s, ok := v.(string); ok {
		return s
	}
	b, err := json.Marshal(v)
	if err != nil {
		return ""
	}
	if len(b) >= 2 && b[0] == '"' && b[len(b)-1] == '"' {
		return string(b[1 : len(b)-1])
	}
	return string(b)
}

func (s *Server) handleAdminGetConfig(c echo.Context) error {
	rows, err := s.db.Query(`SELECT key, value FROM ctf_settings`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	settings := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			continue
		}
		settings[k] = v
	}
	result := map[string]any{
		"ctf_name":          s.cfg.CTF.Name,
		"ctf_description":   "",
		"ctf_start":         "",
		"ctf_end":           "",
		"registration_open": s.cfg.CTF.RegistrationOpen,
		"theme":             "",
		"team_mode":         s.cfg.CTF.TeamMode,
	}
	for k, v := range settings {
		result[k] = normalizeAdminConfigValue(k, v)
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleAdminSetConfig(c echo.Context) error {
	var req map[string]any
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	allowed := map[string]bool{
		"ctf_name": true, "ctf_description": true, "description": true, "ctf_start": true,
		"ctf_end": true, "registration_open": true, "theme": true, "team_mode": true,
		"ctf_theme": true, "theme_header": true, "theme_footer": true, "theme_settings": true,
		"domain_whitelist": true, "verify_emails": true, "team_creation": true, "team_size": true,
		"num_teams": true, "num_users": true, "team_disbanding": true, "incorrect_submissions_per_min": true,
		"name_changes": true, "robots_txt": true,
		"oauth_client_id": true, "oauth_client_secret": true,
		"account_visibility": true, "score_visibility": true, "registration_visibility": true, "paused": true,
		"html_sanitization": true, "registration_code": true,
		"successful_registration_email_subject": true, "successful_registration_email_body": true,
		"verification_email_subject": true, "verification_email_body": true,
		"mailfrom_addr": true, "mail_server": true, "mail_port": true, "mail_username": true, "mail_password": true,
		"mail_useauth": true, "mail_ssl": true, "mail_tls": true,
		"start": true, "end": true, "freeze": true, "view_after_ctf": true,
		"social_shares": true, "tos_text": true, "privacy_text": true, "user_mode": true,
		"notifier_type": true, "notifier_send_notifications": true, "notifier_send_solves": true,
		"notifier_solve_msg": true, "notifier_solve_count": true,
		"notifier_slack_webhook_url": true, "notifier_discord_webhook_url": true,
		"notifier_telegram_bot_token": true, "notifier_telegram_chat_id": true,
		"accounts_verify_emails": true, "accounts_name_changes": true, "accounts_team_creation": true,
		"accounts_max_team_size": true,
		"pages_show_scoreboard":  true, "pages_show_challenges": true, "pages_show_users": true,
		"brackets_enabled": true, "brackets_type": true,
		"customfields_enabled": true, "customfields_required": true,
		"settings_ctf_start": true, "settings_ctf_end": true, "settings_challenge_visibility": true,
		"security_mfa_required": true, "security_max_login_attempts": true, "security_session_timeout_minutes": true,
		"email_from_name": true, "email_from_address": true, "email_smtp_host": true,
		"email_smtp_port": true, "email_smtp_user": true, "email_smtp_pass": true,
		"time_timezone": true, "time_countdown_to": true, "time_freeze_at": true,
		"social_discord": true, "social_twitter": true, "social_website": true,
		"legal_terms_url": true, "legal_privacy_url": true, "legal_rules_url": true,
		"backup_enabled": true, "backup_schedule": true, "backup_retention_days": true,
		"usermode_default": true, "usermode_allow_switch": true,
		"reset_preserve_users": true, "reset_preserve_teams": true,
	}
	for k, v := range req {
		if !allowed[k] {
			continue
		}
		if _, err := s.db.Exec(
			`INSERT INTO ctf_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			k, configValueAsString(v),
		); err != nil {
			if _, err2 := s.db.Exec(`UPDATE ctf_settings SET value=? WHERE key=?`, configValueAsString(v), k); err2 != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "config updated"})
}

func normalizeAdminConfigValue(key, value string) any {
	switch key {
	case "registration_open", "team_mode", "notifier_send_notifications", "notifier_send_solves",
		"verify_emails", "team_creation", "name_changes", "paused",
		"html_sanitization", "mail_useauth", "mail_ssl", "mail_tls", "view_after_ctf", "social_shares",
		"accounts_verify_emails", "accounts_name_changes", "accounts_team_creation",
		"pages_show_scoreboard", "pages_show_challenges", "pages_show_users",
		"brackets_enabled", "customfields_enabled", "customfields_required",
		"security_mfa_required", "backup_enabled", "usermode_allow_switch",
		"reset_preserve_users", "reset_preserve_teams":
		if b, err := strconv.ParseBool(value); err == nil {
			return b
		}
		return value
	case "notifier_solve_count", "team_size", "num_teams", "num_users", "incorrect_submissions_per_min",
		"start", "end", "freeze", "accounts_max_team_size", "security_max_login_attempts",
		"security_session_timeout_minutes", "email_smtp_port", "backup_retention_days":
		if value == "" {
			return nil
		}
		if n, err := strconv.Atoi(value); err == nil {
			return n
		}
		return value
	default:
		return value
	}
}

func configValueAsString(v any) string {
	switch value := v.(type) {
	case string:
		return value
	case bool:
		return strconv.FormatBool(value)
	case float64:
		if value == float64(int64(value)) {
			return strconv.FormatInt(int64(value), 10)
		}
		return strconv.FormatFloat(value, 'f', -1, 64)
	case json.Number:
		return value.String()
	case nil:
		return ""
	default:
		return toString(value)
	}
}

func (s *Server) handleGetTheme(c echo.Context) error {
	var themeJSON string
	err := s.db.QueryRow(`SELECT value FROM ctf_settings WHERE key='theme'`).Scan(&themeJSON)
	if err != nil || themeJSON == "" {
		return c.JSON(http.StatusOK, themeConfig{Name: "default", Vars: map[string]string{}})
	}
	var raw map[string]any
	if err := json.Unmarshal([]byte(themeJSON), &raw); err != nil {
		return c.JSON(http.StatusOK, themeConfig{Name: "default", Vars: map[string]string{}})
	}
	return c.JSON(http.StatusOK, normalizeThemePayload(raw))
}

func (s *Server) handleAdminSetTheme(c echo.Context) error {
	var raw map[string]any
	if err := c.Bind(&raw); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	normalized := normalizeThemePayload(raw)
	b, err := json.Marshal(normalized)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "marshal error"})
	}
	if _, err := s.db.Exec(
		`INSERT INTO ctf_settings (key, value) VALUES ('theme', ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		string(b),
	); err != nil {
		if _, err2 := s.db.Exec(`UPDATE ctf_settings SET value=? WHERE key='theme'`, string(b)); err2 != nil {
			return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
		}
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "theme updated"})
}
