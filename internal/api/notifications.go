package api

import (
	"net/http"
	"strconv"

	"github.com/labstack/echo/v4"

	"github.com/ilyastar9999/heCsTackForse/internal/models"
)

func (s *Server) handleListNotifications(c echo.Context) error {
	rows, err := s.db.Query(`SELECT id, title, content, created_by, created_at FROM notifications ORDER BY created_at DESC`)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	defer rows.Close()
	var notifications []models.Notification
	for rows.Next() {
		var n models.Notification
		if err := rows.Scan(&n.ID, &n.Title, &n.Content, &n.CreatedBy, &n.CreatedAt); err != nil {
			continue
		}
		notifications = append(notifications, n)
	}
	if notifications == nil {
		notifications = []models.Notification{}
	}
	return c.JSON(http.StatusOK, notifications)
}

func (s *Server) handleAdminCreateNotification(c echo.Context) error {
	var req struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	if err := c.Bind(&req); err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid request"})
	}
	if req.Title == "" {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "title is required"})
	}
	userID := getUserID(c)
	id, err := s.db.InsertGetID(
		`INSERT INTO notifications (title, content, created_by) VALUES (?, ?, ?)`,
		req.Title, req.Content, userID,
	)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	var n models.Notification
	_ = s.db.QueryRow(`SELECT id, title, content, created_by, created_at FROM notifications WHERE id=?`, id).
		Scan(&n.ID, &n.Title, &n.Content, &n.CreatedBy, &n.CreatedAt)
	return c.JSON(http.StatusCreated, n)
}

func (s *Server) handleAdminDeleteNotification(c echo.Context) error {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		return c.JSON(http.StatusBadRequest, map[string]string{"error": "invalid id"})
	}
	res, err := s.db.Exec(`DELETE FROM notifications WHERE id=?`, id)
	if err != nil {
		return c.JSON(http.StatusInternalServerError, map[string]string{"error": "db error"})
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return c.JSON(http.StatusNotFound, map[string]string{"error": "not found"})
	}
	return c.JSON(http.StatusOK, map[string]string{"message": "deleted"})
}

// handleAdminListNotifications returns all notifications for the admin view.
func (s *Server) handleAdminListNotifications(c echo.Context) error {
	return s.handleListNotifications(c)
}
