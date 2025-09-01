package authz

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"

	"oddity/pkg/contentstuff"
)

type User struct {
	ID           uint   `gorm:"primaryKey"`
	Username     string `gorm:"uniqueIndex"`
	Email        string `gorm:"uniqueIndex"`
	PasswordHash string
	Role         string    `gorm:"index"`
	CreatedAt    time.Time `gorm:"autoCreateTime"`
	UpdatedAt    time.Time `gorm:"autoUpdateTime"`
}

type UserSession struct {
	ID         uint      `gorm:"primaryKey"`
	UserID     uint      `gorm:"index"`
	Token      string    `gorm:"uniqueIndex"`
	ExpiresAt  time.Time `gorm:"index"`
	User       User      `gorm:"foreignKey:UserID"`
	CustomData string    `gorm:"type:text"` // JSON string for session data
}

type AuthzApp struct {
	SiteContent *contentstuff.ContentStuff
}

func (a *AuthzApp) RegisterRoutes(r *gin.Engine) {
	authGroup := r.Group("/auth")
	{
		authGroup.GET("", a.HandleAuthPage)
		authGroup.POST("/login", a.HandleLogin)
		authGroup.GET("/logout", a.HandleLogoutGET)
		authGroup.POST("/logout", a.HandleLogout)
		authGroup.POST("/change-password", a.HandleChangePassword)
	}
}

func (a *AuthzApp) Init() {
	err := a.SiteContent.DB().AutoMigrate(&User{}, &UserSession{})
	if err != nil {
		log.Fatalf("Failed to migrate authz models: %v", err)
	}

	// Create default admin user if no users exist
	var userCount int64
	a.SiteContent.DB().Model(&User{}).Count(&userCount)
	if userCount == 0 {
		err = a.CreateDefaultAdmin()
		if err != nil {
			log.Fatalf("Failed to create default admin user: %v", err)
		}
		log.Info("Created default admin user: admin / admin")
	}
}

// HashPassword hashes a plaintext password using bcrypt
func (a *AuthzApp) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword compares a plaintext password with a hash
func (a *AuthzApp) CheckPassword(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

// GenerateSessionToken generates a secure random session token
func (a *AuthzApp) GenerateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// CreateDefaultAdmin creates a default admin user
func (a *AuthzApp) CreateDefaultAdmin() error {
	hash, err := a.HashPassword("admin")
	if err != nil {
		return err
	}

	user := User{
		Username:     "admin",
		Email:        "admin@localhost",
		PasswordHash: hash,
		Role:         "admin",
	}

	return a.SiteContent.DB().Create(&user).Error
}

// AuthenticateUser authenticates a user with username/password
func (a *AuthzApp) AuthenticateUser(username, password string) (*User, error) {
	var user User
	err := a.SiteContent.DB().Where("username = ?", username).First(&user).Error
	if err != nil {
		return nil, err
	}

	if !a.CheckPassword(password, user.PasswordHash) {
		return nil, nil
	}

	return &user, nil
}

// CreateSession creates a new session for a user
func (a *AuthzApp) CreateSession(userID uint) (*UserSession, error) {
	token, err := a.GenerateSessionToken()
	if err != nil {
		return nil, err
	}

	session := UserSession{
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(24 * time.Hour),
	}

	err = a.SiteContent.DB().Create(&session).Error
	if err != nil {
		return nil, err
	}

	return &session, nil
}

// GetSessionByToken retrieves a session by token
func (a *AuthzApp) GetSessionByToken(token string) (*UserSession, error) {
	var session UserSession
	err := a.SiteContent.DB().Preload("User").Where("token = ? AND expires_at > ?", token, time.Now()).First(&session).Error
	if err != nil {
		return nil, err
	}

	return &session, nil
}

// DeleteSession deletes a session by token
func (a *AuthzApp) DeleteSession(token string) error {
	return a.SiteContent.DB().Where("token = ?", token).Delete(&UserSession{}).Error
}

// ChangePassword changes a user's password
func (a *AuthzApp) ChangePassword(userID uint, newPassword string) error {
	hash, err := a.HashPassword(newPassword)
	if err != nil {
		return err
	}

	return a.SiteContent.DB().Model(&User{}).Where("id = ?", userID).Update("password_hash", hash).Error
}

// CleanupExpiredSessions removes expired sessions
func (a *AuthzApp) CleanupExpiredSessions() error {
	return a.SiteContent.DB().Where("expires_at < ?", time.Now()).Delete(&UserSession{}).Error
}

// Middleware

// AuthMiddleware creates authentication middleware that checks for valid sessions
func (a *AuthzApp) AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		sessionToken, err := c.Cookie("session_token")
		if err != nil {
			c.Next()
			return
		}

		session, err := a.GetSessionByToken(sessionToken)
		if err != nil {
			c.Next()
			return
		}

		c.Set("authenticated_user", &session.User)
		c.Set("session", session)
		c.Next()
	}
}

// RequireAuth middleware that requires authentication
func (a *AuthzApp) RequireAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		user, exists := c.Get("authenticated_user")
		if !exists || user == nil {
			c.JSON(401, gin.H{"error": "Authentication required"})
			c.Abort()
			return
		}
		c.Next()
	}
}

// GetCurrentUser helper function to get the current authenticated user
func GetCurrentUser(c *gin.Context) (*User, bool) {
	user, exists := c.Get("authenticated_user")
	if !exists {
		return nil, false
	}
	if u, ok := user.(*User); ok {
		return u, true
	}
	return nil, false
}

// IsAuthenticated checks if the current request has an authenticated user
func IsAuthenticated(c *gin.Context) bool {
	_, exists := c.Get("authenticated_user")
	return exists
}

// Route handlers

// HandleAuthPage serves the auth page
func (a *AuthzApp) HandleAuthPage(c *gin.Context) {
	// Check if user is already authenticated
	sessionToken, err := c.Cookie("session_token")
	var currentUser *User
	if err == nil && sessionToken != "" {
		if session, err := a.GetSessionByToken(sessionToken); err == nil {
			currentUser = &session.User
		}
	}

	data := gin.H{}
	if currentUser != nil {
		data["Render"] = "change-password"
		data["Username"] = currentUser.Username
	} else {
		data["Render"] = "login-page"
	}

	c.HTML(http.StatusOK, "auth.html", data)
}

// HandleLogin handles login requests
func (a *AuthzApp) HandleLogin(c *gin.Context) {
	type LoginRequest struct {
		Username string `form:"username" binding:"required"`
		Password string `form:"password" binding:"required"`
	}

	var req LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing username or password"})
		return
	}

	user, err := a.AuthenticateUser(req.Username, req.Password)
	if err != nil || user == nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
		return
	}

	session, err := a.CreateSession(user.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create session"})
		return
	}

	c.SetCookie("session_token", session.Token, 24*60*60, "/", "", false, true)
	c.JSON(http.StatusOK, gin.H{"success": true, "redirect": "/"})
}

// HandleLogout handles logout requests
func (a *AuthzApp) HandleLogout(c *gin.Context) {
	a.doLogout(c)
	c.JSON(http.StatusOK, gin.H{"success": true, "redirect": "/"})
}

func (a *AuthzApp) doLogout(c *gin.Context) {
	sessionToken, err := c.Cookie("session_token")
	if err == nil && sessionToken != "" {
		a.DeleteSession(sessionToken)
	}

	c.SetCookie("session_token", "", -1, "/", "", false, true)
}

// HandleLogoutGET handles logout requests via GET (for convenience)
func (a *AuthzApp) HandleLogoutGET(c *gin.Context) {
	a.doLogout(c)
	c.Redirect(http.StatusSeeOther, "/")
}

// HandleChangePassword handles password change requests
func (a *AuthzApp) HandleChangePassword(c *gin.Context) {
	type ChangePasswordRequest struct {
		CurrentPassword string `form:"current_password" binding:"required"`
		NewPassword     string `form:"new_password" binding:"required"`
	}

	sessionToken, err := c.Cookie("session_token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Not authenticated"})
		return
	}

	session, err := a.GetSessionByToken(sessionToken)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid session"})
		return
	}

	var req ChangePasswordRequest
	if err := c.ShouldBind(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Missing current password or new password"})
		return
	}

	if !a.CheckPassword(req.CurrentPassword, session.User.PasswordHash) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Current password is incorrect"})
		return
	}

	if len(req.NewPassword) < 4 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "New password must be at least 4 characters"})
		return
	}

	err = a.ChangePassword(session.User.ID, req.NewPassword)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to change password"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": true, "message": "Password changed successfully"})
}

// Session Storage Methods

// SetSessionData stores custom data in the session
func (a *AuthzApp) SetSessionData(c *gin.Context, key string, value interface{}) error {
	sessionToken, err := c.Cookie("session_token")
	if err != nil {
		return err
	}

	session, err := a.GetSessionByToken(sessionToken)
	if err != nil {
		return err
	}

	var data map[string]interface{}
	if session.CustomData != "" {
		err = json.Unmarshal([]byte(session.CustomData), &data)
		if err != nil {
			data = make(map[string]interface{})
		}
	} else {
		data = make(map[string]interface{})
	}

	data[key] = value

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return a.SiteContent.DB().Model(&UserSession{}).
		Where("token = ?", sessionToken).
		Update("custom_data", string(jsonData)).Error
}

// GetSessionData retrieves custom data from the session
func (a *AuthzApp) GetSessionData(c *gin.Context, key string) (interface{}, bool) {
	sessionToken, err := c.Cookie("session_token")
	if err != nil {
		return nil, false
	}

	session, err := a.GetSessionByToken(sessionToken)
	if err != nil {
		return nil, false
	}

	if session.CustomData == "" {
		return nil, false
	}

	var data map[string]interface{}
	err = json.Unmarshal([]byte(session.CustomData), &data)
	if err != nil {
		return nil, false
	}

	value, exists := data[key]
	return value, exists
}

// DeleteSessionData removes a key from session custom data
func (a *AuthzApp) DeleteSessionData(c *gin.Context, key string) error {
	sessionToken, err := c.Cookie("session_token")
	if err != nil {
		return err
	}

	session, err := a.GetSessionByToken(sessionToken)
	if err != nil {
		return err
	}

	if session.CustomData == "" {
		return nil // nothing to delete
	}

	var data map[string]interface{}
	err = json.Unmarshal([]byte(session.CustomData), &data)
	if err != nil {
		return err
	}

	delete(data, key)

	jsonData, err := json.Marshal(data)
	if err != nil {
		return err
	}

	return a.SiteContent.DB().Model(&UserSession{}).
		Where("token = ?", sessionToken).
		Update("custom_data", string(jsonData)).Error
}

// SetFlash Flash message helpers
func (a *AuthzApp) SetFlash(c *gin.Context, message string) error {
	return a.SetSessionData(c, "flash_message", message)
}

func (a *AuthzApp) GetFlash(c *gin.Context) string {
	if value, exists := a.GetSessionData(c, "flash_message"); exists {
		// Auto-delete flash message after reading
		a.DeleteSessionData(c, "flash_message")
		if msg, ok := value.(string); ok {
			return msg
		}
	}
	return ""
}
