package models

import "time"

type Challenge struct {
	ID            int64     `json:"id" db:"id"`
	Name          string    `json:"name" db:"name"`
	Description   string    `json:"description" db:"description"`
	Category      string    `json:"category" db:"category"`
	Points        int       `json:"points" db:"points"`
	Flag          string    `json:"flag,omitempty" db:"flag"`
	FlagType      string    `json:"flag_type" db:"flag_type"`
	DeployType    string    `json:"deploy_type" db:"deploy_type"`
	DeployBackend string    `json:"deploy_backend" db:"deploy_backend"`
	DeployConfig  string    `json:"deploy_config" db:"deploy_config"`
	Image         string    `json:"image" db:"image"`
	VMTemplate    int       `json:"vm_template" db:"vm_template"`
	IsVisible     bool      `json:"is_visible" db:"is_visible"`
	ConnectionInfo string `json:"connection_info" db:"connection_info"`
	MaxAttempts   int    `json:"max_attempts" db:"max_attempts"`
	CreatedAt     time.Time `json:"created_at" db:"created_at"`
	SolveCount    int       `json:"solve_count,omitempty"`
	Solved        bool      `json:"solved,omitempty"`
}
