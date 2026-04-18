package api

import (
	"net/http"

	"github.com/labstack/echo/v4"
)

type CategoryStat struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Solved int    `json:"solved"`
}

func (s *Server) handleStatistics(c echo.Context) error {
	var usersCount, teamsCount, challengesCount, submissionsCount, correctSubmissions int

	_ = s.db.QueryRow(`SELECT COUNT(*) FROM users`).Scan(&usersCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM teams`).Scan(&teamsCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM challenges`).Scan(&challengesCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions`).Scan(&submissionsCount)
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE is_correct=1`).Scan(&correctSubmissions)

	rows, err := s.db.Query(`SELECT category, COUNT(*) FROM challenges GROUP BY category ORDER BY category`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	var categories []CategoryStat
	for rows.Next() {
		var cat CategoryStat
		if err := rows.Scan(&cat.Name, &cat.Count); err != nil {
			continue
		}
		_ = s.db.QueryRow(
			`SELECT COUNT(DISTINCT challenge_id) FROM submissions s JOIN challenges c ON s.challenge_id=c.id WHERE c.category=? AND s.is_correct=1`,
			cat.Name,
		).Scan(&cat.Solved)
		categories = append(categories, cat)
	}
	if categories == nil {
		categories = []CategoryStat{}
	}

	return c.JSON(http.StatusOK, map[string]any{
		"users_count":         usersCount,
		"teams_count":         teamsCount,
		"challenges_count":    challengesCount,
		"submissions_count":   submissionsCount,
		"correct_submissions": correctSubmissions,
		"categories":          categories,
	})
}
