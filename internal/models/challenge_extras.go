package models

type ChallengeFlag struct {
	ID          int64  `json:"id" db:"id"`
	ChallengeID int64  `json:"challenge_id" db:"challenge_id"`
	Content     string `json:"content" db:"content"`
	Type        string `json:"type" db:"type"`
	Data        string `json:"data" db:"data"`
}

type ChallengeFile struct {
	ID          int64  `json:"id" db:"id"`
	ChallengeID int64  `json:"challenge_id" db:"challenge_id"`
	Name        string `json:"name" db:"name"`
	Location    string `json:"location" db:"location"`
	Size        int64  `json:"size" db:"size"`
}

type ChallengeHint struct {
	ID          int64  `json:"id" db:"id"`
	ChallengeID int64  `json:"challenge_id" db:"challenge_id"`
	Content     string `json:"content" db:"content"`
	Cost        int    `json:"cost" db:"cost"`
	SortOrder   int    `json:"sort_order" db:"sort_order"`
}
