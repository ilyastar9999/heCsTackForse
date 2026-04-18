package models

import "time"

type Page struct {
	ID           int64     `json:"id" db:"id"`
	Title        string    `json:"title" db:"title"`
	Slug         string    `json:"slug" db:"slug"`
	Content      string    `json:"content" db:"content"`
	Draft        bool      `json:"draft" db:"draft"`
	AuthRequired bool      `json:"auth_required" db:"auth_required"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
}
