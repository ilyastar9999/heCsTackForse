package api

import (
	"database/sql"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) handleListModules(c echo.Context) error {
	modules, err := s.loadModules(false, 0, "")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, modules)
}

func (s *Server) handleGetModule(c echo.Context) error {
	modules, err := s.loadModules(false, 0, c.Param("slug"))
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if len(modules) == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	return c.JSON(http.StatusOK, modules[0])
}

func (s *Server) handleAdminListModules(c echo.Context) error {
	modules, err := s.loadModules(true, 0, "")
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, modules)
}

func (s *Server) handleAdminCreateModule(c echo.Context) error {
	var req models.Module
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Slug) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "title and slug are required"})
	}
	id, err := s.db.InsertGetID(
		`INSERT INTO modules (title, slug, description, sort_order, draft) VALUES (?, ?, ?, ?, ?)`,
		strings.TrimSpace(req.Title), strings.TrimSpace(req.Slug), req.Description, req.SortOrder, req.Draft,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	req.ID = id
	if req.Items == nil {
		req.Items = []models.ModuleItem{}
	}
	return c.JSON(http.StatusCreated, req)
}

func (s *Server) handleAdminUpdateModule(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	var req models.Module
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if strings.TrimSpace(req.Title) == "" || strings.TrimSpace(req.Slug) == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "title and slug are required"})
	}
	_, err = s.db.Exec(
		`UPDATE modules SET title=?, slug=?, description=?, sort_order=?, draft=?, updated_at=? WHERE id=?`,
		strings.TrimSpace(req.Title), strings.TrimSpace(req.Slug), req.Description, req.SortOrder, req.Draft, time.Now(), id,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	req.ID = id
	return c.JSON(http.StatusOK, req)
}

func (s *Server) handleAdminDeleteModule(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	if _, err := s.db.Exec(`DELETE FROM module_items WHERE module_id=?`, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	if _, err := s.db.Exec(`DELETE FROM modules WHERE id=?`, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

func (s *Server) handleAdminCreateModuleItem(c echo.Context) error {
	moduleID, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid module id"})
	}
	var req models.ModuleItem
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	req.ModuleID = moduleID
	if err := normalizeModuleItem(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	id, err := s.db.InsertGetID(
		`INSERT INTO module_items (module_id, type, title, content, page_id, challenge_id, sort_order) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		req.ModuleID, req.Type, req.Title, req.Content, nullableInt64(req.PageID), nullableInt64(req.ChallengeID), req.SortOrder,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	req.ID = id
	return c.JSON(http.StatusCreated, req)
}

func (s *Server) handleAdminUpdateModuleItem(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid item id"})
	}
	var req models.ModuleItem
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if err := normalizeModuleItem(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	_, err = s.db.Exec(
		`UPDATE module_items SET type=?, title=?, content=?, page_id=?, challenge_id=?, sort_order=? WHERE id=?`,
		req.Type, req.Title, req.Content, nullableInt64(req.PageID), nullableInt64(req.ChallengeID), req.SortOrder, id,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	req.ID = id
	return c.JSON(http.StatusOK, req)
}

func (s *Server) handleAdminDeleteModuleItem(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("item_id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid item id"})
	}
	if _, err := s.db.Exec(`DELETE FROM module_items WHERE id=?`, id); err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

func (s *Server) loadModules(includeDraft bool, moduleID int64, slug string) ([]models.Module, error) {
	query := `SELECT id, title, slug, description, sort_order, draft, created_at, updated_at FROM modules`
	args := []any{}
	clauses := []string{}
	if !includeDraft {
		clauses = append(clauses, `draft=FALSE`)
	}
	if moduleID > 0 {
		clauses = append(clauses, `id=?`)
		args = append(args, moduleID)
	}
	if slug != "" {
		clauses = append(clauses, `slug=?`)
		args = append(args, slug)
	}
	if len(clauses) > 0 {
		query += ` WHERE ` + strings.Join(clauses, ` AND `)
	}
	query += ` ORDER BY sort_order, id`

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var modules []models.Module
	for rows.Next() {
		var m models.Module
		if err := rows.Scan(&m.ID, &m.Title, &m.Slug, &m.Description, &m.SortOrder, &m.Draft, &m.CreatedAt, &m.UpdatedAt); err != nil {
			return nil, err
		}
		items, err := s.loadModuleItems(m.ID, includeDraft)
		if err != nil {
			return nil, err
		}
		m.Items = items
		modules = append(modules, m)
	}
	if modules == nil {
		modules = []models.Module{}
	}
	return modules, rows.Err()
}

func (s *Server) loadModuleItems(moduleID int64, includeDraft bool) ([]models.ModuleItem, error) {
	rows, err := s.db.Query(`SELECT id, module_id, type, title, content, page_id, challenge_id, sort_order FROM module_items WHERE module_id=? ORDER BY sort_order, id`, moduleID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []models.ModuleItem
	for rows.Next() {
		var item models.ModuleItem
		var pageID sql.NullInt64
		var challengeID sql.NullInt64
		if err := rows.Scan(&item.ID, &item.ModuleID, &item.Type, &item.Title, &item.Content, &pageID, &challengeID, &item.SortOrder); err != nil {
			return nil, err
		}
		if pageID.Valid {
			item.PageID = &pageID.Int64
			page, ok, err := s.loadModulePage(pageID.Int64, includeDraft)
			if err != nil {
				return nil, err
			}
			if !ok && !includeDraft {
				continue
			}
			if ok {
				item.Page = &page
				if item.Title == "" {
					item.Title = page.Title
				}
			}
		}
		if challengeID.Valid {
			item.ChallengeID = &challengeID.Int64
			ch, ok, err := s.loadModuleChallenge(challengeID.Int64, includeDraft)
			if err != nil {
				return nil, err
			}
			if !ok && !includeDraft {
				continue
			}
			if ok {
				item.Challenge = &ch
				if item.Title == "" {
					item.Title = ch.Name
				}
			}
		}
		items = append(items, item)
	}
	if items == nil {
		items = []models.ModuleItem{}
	}
	return items, rows.Err()
}

func (s *Server) loadModulePage(id int64, includeDraft bool) (models.Page, bool, error) {
	query := `SELECT id, title, slug, content, draft, auth_required, created_at, updated_at FROM pages WHERE id=?`
	if !includeDraft {
		query += ` AND draft=FALSE AND auth_required=FALSE`
	}
	var p models.Page
	err := s.db.QueryRow(query, id).Scan(&p.ID, &p.Title, &p.Slug, &p.Content, &p.Draft, &p.AuthRequired, &p.CreatedAt, &p.UpdatedAt)
	if err == sql.ErrNoRows {
		return p, false, nil
	}
	return p, err == nil, err
}

func (s *Server) loadModuleChallenge(id int64, includeHidden bool) (models.Challenge, bool, error) {
	query := `SELECT id, name, description, category, points, challenge_type, flag_type, deploy_type, deploy_backend, is_visible, connection_info, created_at FROM challenges WHERE id=?`
	if !includeHidden {
		query += ` AND is_visible`
	}
	var ch models.Challenge
	err := s.db.QueryRow(query, id).Scan(&ch.ID, &ch.Name, &ch.Description, &ch.Category, &ch.Points, &ch.ChallengeType, &ch.FlagType, &ch.DeployType, &ch.DeployBackend, &ch.IsVisible, &ch.ConnectionInfo, &ch.CreatedAt)
	if err == sql.ErrNoRows {
		return ch, false, nil
	}
	if err != nil {
		return ch, false, err
	}
	var solveCount int
	_ = s.db.QueryRow(`SELECT COUNT(*) FROM submissions WHERE challenge_id=? AND is_correct`, id).Scan(&solveCount)
	ch.SolveCount = solveCount
	ch.Points = s.scoreChallenge(&ch, solveCount)
	return ch, true, nil
}

func normalizeModuleItem(item *models.ModuleItem) error {
	item.Type = strings.TrimSpace(strings.ToLower(item.Type))
	item.Title = strings.TrimSpace(item.Title)
	switch item.Type {
	case "theory":
		item.Content = strings.TrimSpace(item.Content)
		item.ChallengeID = nil
	case "challenge":
		if item.ChallengeID == nil || *item.ChallengeID <= 0 {
			return fmt.Errorf("challenge_id is required for challenge")
		}
		item.PageID = nil
	default:
		return fmt.Errorf("type must be theory or challenge")
	}
	return nil
}

func nullableInt64(v *int64) any {
	if v == nil || *v <= 0 {
		return nil
	}
	return *v
}
