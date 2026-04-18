package api

import (
	"encoding/json"
	"net/http"

	"github.com/labstack/echo/v4"
)

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
		result[k] = v
	}
	return c.JSON(http.StatusOK, result)
}

func (s *Server) handleAdminSetConfig(c echo.Context) error {
	var req map[string]string
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	allowed := map[string]bool{
		"ctf_name": true, "ctf_description": true, "ctf_start": true,
		"ctf_end": true, "registration_open": true, "theme": true, "team_mode": true,
	}
	for k, v := range req {
		if !allowed[k] {
			continue
		}
		if _, err := s.db.Exec(
			`INSERT INTO ctf_settings (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			k, v,
		); err != nil {
			if _, err2 := s.db.Exec(`UPDATE ctf_settings SET value=? WHERE key=?`, v, k); err2 != nil {
				return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
			}
		}
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "config updated"})
}

func (s *Server) handleGetTheme(c echo.Context) error {
	var themeJSON string
	err := s.db.QueryRow(`SELECT value FROM ctf_settings WHERE key='theme'`).Scan(&themeJSON)
	if err != nil || themeJSON == "" {
		return c.JSON(http.StatusOK, map[string]any{})
	}
	var vars map[string]any
	if err := json.Unmarshal([]byte(themeJSON), &vars); err != nil {
		return c.JSON(http.StatusOK, map[string]any{})
	}
	return c.JSON(http.StatusOK, vars)
}

func (s *Server) handleAdminSetTheme(c echo.Context) error {
	var req struct {
		Vars map[string]string `json:"vars"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	b, err := json.Marshal(req.Vars)
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
