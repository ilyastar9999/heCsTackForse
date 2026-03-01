package models

import "time"

type User struct {
	ID           int64     `json:"id" db:"id"`
	Username     string    `json:"username" db:"username"`
	Email        string    `json:"email,omitempty" db:"email"`
	PasswordHash string    `json:"-" db:"password_hash"`
	Role         string    `json:"role" db:"role"`
	Score        int       `json:"score" db:"score"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
}
