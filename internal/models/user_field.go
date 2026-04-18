package models

type UserField struct {
	ID          int64  `json:"id" db:"id"`
	Name        string `json:"name" db:"name"`
	FieldType   string `json:"field_type" db:"field_type"`
	Required    bool   `json:"required" db:"required"`
	Public      bool   `json:"public" db:"public"`
	Description string `json:"description" db:"description"`
}

type UserFieldValue struct {
	ID      int64  `json:"id" db:"id"`
	UserID  int64  `json:"user_id" db:"user_id"`
	FieldID int64  `json:"field_id" db:"field_id"`
	Value   string `json:"value" db:"value"`
}
