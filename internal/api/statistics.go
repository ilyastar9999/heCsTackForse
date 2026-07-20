package api

import (
	"context"
	"net/http"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/cache"
)

type CategoryStat struct {
	Name   string `json:"name"`
	Count  int    `json:"count"`
	Solved int    `json:"solved"`
}

func (s *Server) handleStatistics(c echo.Context) error {
	ttl := cache.ParseDuration(s.cfg.Cache.TTLStatistics, 60*time.Second)
	cacheKey := cache.Key("statistics", "summary")

	type cachedStats struct {
		UsersCount         int            `json:"users_count"`
		TeamsCount         int            `json:"teams_count"`
		ChallengesCount    int            `json:"challenges_count"`
		SubmissionsCount   int            `json:"submissions_count"`
		CorrectSubmissions int            `json:"correct_submissions"`
		Categories         []CategoryStat `json:"categories"`
	}

	var cached cachedStats
	if s.cache.Get(context.Background(), cacheKey, &cached) {
		return c.JSON(http.StatusOK, map[string]any{
			"users_count":         cached.UsersCount,
			"teams_count":         cached.TeamsCount,
			"challenges_count":    cached.ChallengesCount,
			"submissions_count":   cached.SubmissionsCount,
			"correct_submissions": cached.CorrectSubmissions,
			"categories":          cached.Categories,
		})
	}

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

	result := cachedStats{
		UsersCount:         usersCount,
		TeamsCount:         teamsCount,
		ChallengesCount:    challengesCount,
		SubmissionsCount:   submissionsCount,
		CorrectSubmissions: correctSubmissions,
		Categories:         categories,
	}
	s.cache.Set(context.Background(), cacheKey, result, ttl)

	return c.JSON(http.StatusOK, map[string]any{
		"users_count":         usersCount,
		"teams_count":         teamsCount,
		"challenges_count":    challengesCount,
		"submissions_count":   submissionsCount,
		"correct_submissions": correctSubmissions,
		"categories":          categories,
	})
}

// InvalidateStatisticsCache removes cached statistics data.
func (s *Server) InvalidateStatisticsCache() {
	if s.cache != nil {
		s.cache.InvalidatePrefix(context.Background(), "statistics:*")
	}
}
