package models

// UserRole model
type UserRole struct {
	ID      uint32 `json:"id,omitempty"`
	Name    string `json:"name,omitempty" gorm:"unique_index;size:64"`
	Enabled bool   `json:"enabled,omitempty"`
}
