package models

import "time"

type Submission struct {
	ID          int64     `json:"id" db:"id"`
	UserID      int64     `json:"user_id" db:"user_id"`
	TeamID      *int64    `json:"team_id,omitempty" db:"team_id"`
	ChallengeID int64     `json:"challenge_id" db:"challenge_id"`
	Flag        string    `json:"flag" db:"flag"`
	IsCorrect   bool      `json:"is_correct" db:"is_correct"`
	IP          string    `json:"ip" db:"ip"`
	SubmittedAt time.Time `json:"submitted_at" db:"submitted_at"`
}

type Instance struct {
	ID             int64      `json:"id" db:"id"`
	ChallengeID    int64      `json:"challenge_id" db:"challenge_id"`
	UserID         *int64     `json:"user_id,omitempty" db:"user_id"`
	TeamID         *int64     `json:"team_id,omitempty" db:"team_id"`
	InstanceType   string     `json:"instance_type" db:"instance_type"`
	Backend        string     `json:"backend" db:"backend"`
	InstanceID     string     `json:"instance_id" db:"instance_id"`
	ConnectionInfo string     `json:"connection_info" db:"connection_info"`
	CreatedAt      time.Time  `json:"created_at" db:"created_at"`
	ExpiresAt      *time.Time `json:"expires_at,omitempty" db:"expires_at"`
	Status         string     `json:"status" db:"status"`
}

type ADFlag struct {
	ID          int64     `json:"id" db:"id"`
	ChallengeID int64     `json:"challenge_id" db:"challenge_id"`
	TeamID      int64     `json:"team_id" db:"team_id"`
	Flag        string    `json:"flag" db:"flag"`
	Round       int       `json:"round" db:"round"`
	CreatedAt   time.Time `json:"created_at" db:"created_at"`
}

type ADService struct {
	ID          int64     `json:"id" db:"id"`
	ChallengeID int64     `json:"challenge_id" db:"challenge_id"`
	TeamID      int64     `json:"team_id" db:"team_id"`
	Status      string    `json:"status" db:"status"`
	Round       int       `json:"round" db:"round"`
	Score       int       `json:"score" db:"score"`
	CheckedAt   time.Time `json:"checked_at" db:"checked_at"`
}
