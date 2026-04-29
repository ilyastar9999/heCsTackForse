package api

import (
	"database/sql"
	"net/http"
	"strconv"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) handleListPages(c echo.Context) error {
	query := `SELECT id, title, slug, content, draft, auth_required, created_at, updated_at FROM pages WHERE draft=0`
	if _, _, err := s.authenticateRequest(c); err != nil {
		query += ` AND auth_required=0`
	}
	query += ` ORDER BY id`
	rows, err := s.db.Query(query)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var pages []models.Page
	for rows.Next() {
		var p models.Page
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.Draft, &p.AuthRequired, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		pages = append(pages, p)
	}
	if pages == nil {
		pages = []models.Page{}
	}
	return c.JSON(http.StatusOK, pages)
}

func (s *Server) handleGetPage(c echo.Context) error {
	slug := c.Param("slug")
	var p models.Page
	err := s.db.QueryRow(`SELECT id, title, slug, content, draft, auth_required, created_at, updated_at FROM pages WHERE slug=? AND draft=0`, slug).
		Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.Draft, &p.AuthRequired, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if p.AuthRequired {
		if _, _, err := s.authenticateRequest(c); err != nil {
			return c.JSON(http.StatusUnauthorized, map[string]string{"error": "authentication required"})
		}
	}
	return c.JSON(http.StatusOK, p)
}

func (s *Server) handleAdminListPages(c echo.Context) error {
	rows, err := s.db.Query(`SELECT id, title, slug, content, draft, auth_required, created_at, updated_at FROM pages ORDER BY id`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var pages []models.Page
	for rows.Next() {
		var p models.Page
		if err := rows.Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.Draft, &p.AuthRequired, &p.CreatedAt, &p.UpdatedAt); err != nil {
			continue
		}
		pages = append(pages, p)
	}
	if pages == nil {
		pages = []models.Page{}
	}
	return c.JSON(http.StatusOK, pages)
}

func (s *Server) handleAdminCreatePage(c echo.Context) error {
	var req struct {
		Title        string `json:"title"`
		Slug         string `json:"slug"`
		Content      string `json:"content"`
		Draft        bool   `json:"draft"`
		AuthRequired bool   `json:"auth_required"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Title == "" || req.Slug == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "title and slug are required"})
	}
	id, err := s.db.InsertGetID(
		`INSERT INTO pages (title, slug, content, draft, auth_required) VALUES (?, ?, ?, ?, ?)`,
		req.Title, req.Slug, req.Content, req.Draft, req.AuthRequired,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error: " + err.Error()})
	}
	var p models.Page
	_ = s.db.QueryRow(`SELECT id, title, slug, content, draft, auth_required, created_at, updated_at FROM pages WHERE id=?`, id).
		Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.Draft, &p.AuthRequired, &p.CreatedAt, &p.UpdatedAt)
	return c.JSON(http.StatusCreated, p)
}

func (s *Server) handleAdminUpdatePage(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var req struct {
		Title        string `json:"title"`
		Slug         string `json:"slug"`
		Content      string `json:"content"`
		Draft        bool   `json:"draft"`
		AuthRequired bool   `json:"auth_required"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	now := time.Now()
	_, err = s.db.Exec(
		`UPDATE pages SET title=?, slug=?, content=?, draft=?, auth_required=?, updated_at=? WHERE id=?`,
		req.Title, req.Slug, req.Content, req.Draft, req.AuthRequired, now, id,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "updated"})
}

func (s *Server) handleAdminDeletePage(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	_, err = s.db.Exec(`DELETE FROM pages WHERE id=?`, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}
