package models

import "time"

type Module struct {
	ID          int64        `json:"id" db:"id"`
	Title       string       `json:"title" db:"title"`
	Slug        string       `json:"slug" db:"slug"`
	Description string       `json:"description" db:"description"`
	SortOrder   int          `json:"sort_order" db:"sort_order"`
	Draft       bool         `json:"draft" db:"draft"`
	CreatedAt   time.Time    `json:"created_at" db:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at" db:"updated_at"`
	Items       []ModuleItem `json:"items,omitempty"`
}

type ModuleItem struct {
	ID          int64      `json:"id" db:"id"`
	ModuleID    int64      `json:"module_id" db:"module_id"`
	Type        string     `json:"type" db:"type"`
	Title       string     `json:"title" db:"title"`
	Content     string     `json:"content" db:"content"`
	PageID      *int64     `json:"page_id,omitempty" db:"page_id"`
	ChallengeID *int64     `json:"challenge_id,omitempty" db:"challenge_id"`
	SortOrder   int        `json:"sort_order" db:"sort_order"`
	Page        *Page      `json:"page,omitempty"`
	Challenge   *Challenge `json:"challenge,omitempty"`
}
