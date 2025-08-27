package token

import (
	"fmt"
	"time"

	"blogsync2/pkg/db"

	"gorm.io/gorm"
)

type DBStorage struct {
	database *gorm.DB
	userID   uint
}

func NewDBStorage(database *gorm.DB, userID uint) *DBStorage {
	return &DBStorage{
		database: database,
		userID:   userID,
	}
}

func (d *DBStorage) SaveToken(accessToken, refreshToken string, expiresAt time.Time) error {
	token := &db.Token{
		UserID:       d.userID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}

	result := d.database.Where("user_id = ?", d.userID).First(&db.Token{})
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			return d.database.Create(token).Error
		}
		return fmt.Errorf("failed to check existing token: %w", result.Error)
	}

	return d.database.Where("user_id = ?", d.userID).Updates(token).Error
}

func (d *DBStorage) LoadToken() (*TokenData, error) {
	var token db.Token
	if err := d.database.Where("user_id = ?", d.userID).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("token not found for user")
		}
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	return &TokenData{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
	}, nil
}

func (d *DBStorage) HasValidToken() bool {
	tokenData, err := d.LoadToken()
	if err != nil {
		return false
	}

	return time.Now().Add(5 * time.Minute).Before(tokenData.ExpiresAt)
}

type DBStorageWithUserCreation struct {
	database *gorm.DB
	userID   *uint
}

func NewDBStorageWithUserCreation(database *gorm.DB) *DBStorageWithUserCreation {
	return &DBStorageWithUserCreation{
		database: database,
		userID:   nil,
	}
}

func (d *DBStorageWithUserCreation) SaveToken(accessToken, refreshToken string, expiresAt time.Time) error {
	return fmt.Errorf("use SaveTokenWithUser instead - this storage requires user creation")
}

func (d *DBStorageWithUserCreation) SaveTokenWithUser(accessToken, refreshToken string, expiresAt time.Time, accountID, displayName, email string) error {
	tx := d.database.Begin()
	if tx.Error != nil {
		return fmt.Errorf("failed to begin transaction: %w", tx.Error)
	}
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	var user db.User
	result := tx.Where("account_id = ?", accountID).First(&user)
	if result.Error != nil {
		if result.Error == gorm.ErrRecordNotFound {
			user = db.User{
				AccountID:   accountID,
				DisplayName: displayName,
				Email:       email,
			}
			if err := tx.Create(&user).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create user: %w", err)
			}
		} else {
			tx.Rollback()
			return fmt.Errorf("failed to check existing user: %w", result.Error)
		}
	} else {
		user.DisplayName = displayName
		user.Email = email
		if err := tx.Save(&user).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update user: %w", err)
		}
	}

	d.userID = &user.ID

	token := &db.Token{
		UserID:       user.ID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		ExpiresAt:    expiresAt,
	}

	var existingToken db.Token
	tokenResult := tx.Where("user_id = ?", user.ID).First(&existingToken)
	if tokenResult.Error != nil {
		if tokenResult.Error == gorm.ErrRecordNotFound {
			if err := tx.Create(token).Error; err != nil {
				tx.Rollback()
				return fmt.Errorf("failed to create token: %w", err)
			}
		} else {
			tx.Rollback()
			return fmt.Errorf("failed to check existing token: %w", tokenResult.Error)
		}
	} else {
		if err := tx.Where("user_id = ?", user.ID).Updates(token).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("failed to update token: %w", err)
		}
	}

	return tx.Commit().Error
}

func (d *DBStorageWithUserCreation) LoadToken() (*TokenData, error) {
	if d.userID == nil {
		var user db.User
		if err := d.database.First(&user).Error; err != nil {
			return nil, fmt.Errorf("no user found and userID not set")
		}
		d.userID = &user.ID
	}

	var token db.Token
	if err := d.database.Where("user_id = ?", *d.userID).First(&token).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("token not found for user")
		}
		return nil, fmt.Errorf("failed to load token: %w", err)
	}

	return &TokenData{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
	}, nil
}

func (d *DBStorageWithUserCreation) HasValidToken() bool {
	tokenData, err := d.LoadToken()
	if err != nil {
		return false
	}

	return time.Now().Add(5 * time.Minute).Before(tokenData.ExpiresAt)
}