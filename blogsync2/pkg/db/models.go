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

type File struct {
	ID          uint      `gorm:"primarykey" json:"id"`
	UserID      uint      `gorm:"not null;index:idx_user" json:"user_id"`
	FileID      string    `gorm:"index:idx_file_id" json:"file_id"`
	RemotePath  string    `gorm:"index:idx_remote_path" json:"remote_path"`
	LocalPath   string    `gorm:"not null;index:idx_local_path" json:"localpath"`
	ContentHash string    `gorm:"not null" json:"content_hash"`
	ModifiedAt  time.Time `json:"modified_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Size        uint64    `gorm:"not null" json:"size"`
}

type Token struct {
	ID           uint      `gorm:"primarykey" json:"id"`
	UserID       uint      `gorm:"uniqueIndex;not null" json:"user_id"`
	AccessToken  string    `gorm:"not null" json:"access_token"`
	RefreshToken string    `json:"refresh_token"`
	ExpiresAt    time.Time `gorm:"not null" json:"expires_at"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	User         User      `gorm:"foreignKey:UserID"`
}

func Migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{}, &SyncCursor{}, &File{}, &Token{})
}
