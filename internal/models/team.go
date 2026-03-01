package models

import "time"

type Team struct {
	ID         int64     `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`
	InviteCode string    `json:"invite_code,omitempty" db:"invite_code"`
	Score      int       `json:"score" db:"score"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}

type TeamMember struct {
	UserID   int64     `json:"user_id" db:"user_id"`
	TeamID   int64     `json:"team_id" db:"team_id"`
	JoinedAt time.Time `json:"joined_at" db:"joined_at"`
}
