package api

import (
	"net/http"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

type ScoreboardEntry struct {
	Rank   int          `json:"rank"`
	User   *models.User `json:"user,omitempty"`
	Team   *models.Team `json:"team,omitempty"`
	Score  int          `json:"score"`
	Solves int          `json:"solves"`
}

func (s *Server) handleScoreboard(c echo.Context) error {
	if s.cfg.CTF.TeamMode {
		return s.handleTeamScoreboard(c)
	}
	return s.handleUserScoreboard(c)
}

func (s *Server) handleUserScoreboard(c echo.Context) error {
	rows, err := s.db.Query(
		`SELECT u.id, u.username, u.role, u.score, u.created_at,
		(SELECT COUNT(*) FROM submissions WHERE user_id=u.id AND is_correct=1) as solves
		FROM users u ORDER BY u.score DESC, u.created_at ASC LIMIT 100`,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var entries []ScoreboardEntry
	rank := 1
	for rows.Next() {
		var u models.User
		var solves int
		if err := rows.Scan(&u.ID, &u.Username, &u.Role, &u.Score, &u.CreatedAt, &solves); err != nil {
			continue
		}
		entries = append(entries, ScoreboardEntry{Rank: rank, User: &u, Score: u.Score, Solves: solves})
		rank++
	}
	if entries == nil {
		entries = []ScoreboardEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}

func (s *Server) handleTeamScoreboard(c echo.Context) error {
	rows, err := s.db.Query(
		`SELECT t.id, t.name, t.score, t.created_at,
		(SELECT COUNT(DISTINCT s.challenge_id) FROM submissions s JOIN team_members tm ON s.user_id=tm.user_id WHERE tm.team_id=t.id AND s.is_correct=1) as solves
		FROM teams t ORDER BY t.score DESC, t.created_at ASC LIMIT 100`,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var entries []ScoreboardEntry
	rank := 1
	for rows.Next() {
		var t models.Team
		var solves int
		if err := rows.Scan(&t.ID, &t.Name, &t.Score, &t.CreatedAt, &solves); err != nil {
			continue
		}
		entries = append(entries, ScoreboardEntry{Rank: rank, Team: &t, Score: t.Score, Solves: solves})
		rank++
	}
	if entries == nil {
		entries = []ScoreboardEntry{}
	}
	return c.JSON(http.StatusOK, entries)
}
