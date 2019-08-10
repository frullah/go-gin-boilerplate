package models

// User model
type User struct {
	ID       *uint64   `json:"id,omitempty"`
	Email    string    `json:"email,omitempty" gorm:"unique_index;size:128;not null"`
	Username string    `json:"username,omitempty" gorm:"unique_index;size:64;not null"`
	Password string    `json:"-" gorm:"size:64;not null"`
	Name     string    `json:"name,omitempty" gorm:"size:64;not null"`
	Role     *UserRole `json:"role,omitempty" gorm:"foreignkey:RoleID"`
	RoleID   uint32    `json:"-"`
	Enabled  bool      `json:"enabled,omitempty" gorm:"not null"`
	Verified bool      `json:"verified,omitempty" gorm:"not null"`
}
