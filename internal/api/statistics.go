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

	if err := s.db.QueryRow(`
		SELECT
			(SELECT COUNT(*) FROM users),
			(SELECT COUNT(*) FROM teams),
			(SELECT COUNT(*) FROM challenges),
			(SELECT COUNT(*) FROM submissions),
			(SELECT COUNT(*) FROM submissions WHERE is_correct)
	`).Scan(&usersCount, &teamsCount, &challengesCount, &submissionsCount, &correctSubmissions); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}

	rows, err := s.db.Query(`
		SELECT c.category,
		       COUNT(*) AS challenge_count,
		       COALESCE(sol.solved_count, 0) AS solved_count
		  FROM challenges c
		  LEFT JOIN (
		      SELECT c2.category, COUNT(DISTINCT s.challenge_id) AS solved_count
		        FROM submissions s
		        JOIN challenges c2 ON c2.id = s.challenge_id
		       WHERE s.is_correct
		       GROUP BY c2.category
		  ) sol ON sol.category = c.category
		 GROUP BY c.category, sol.solved_count
		 ORDER BY c.category
	`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()

	var categories []CategoryStat
	for rows.Next() {
		var cat CategoryStat
		if err := rows.Scan(&cat.Name, &cat.Count, &cat.Solved); err != nil {
			continue
		}
		categories = append(categories, cat)
	}
	if err := rows.Err(); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
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
