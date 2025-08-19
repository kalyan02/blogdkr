package db

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	AccountID   string    `gorm:"uniqueIndex;not null" json:"account_id"`
	DisplayName string    `json:"display_name"`
	Email       string    `json:"email"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type SyncCursor struct {
	ID        uint      `gorm:"primarykey" json:"id"`
	UserID    uint      `gorm:"not null" json:"user_id"`
	Cursor    string    `gorm:"not null" json:"cursor"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	User      User      `gorm:"foreignKey:UserID"`
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &SyncCursor{})
}
