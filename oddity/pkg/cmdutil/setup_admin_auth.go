package cmdutil

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/ssh/terminal"

	"oddity/pkg/authz"
	"oddity/pkg/config"
	"oddity/pkg/contentstuff"
)

func RunAdminAuthSetup(configPath string) {
	// Load configuration
	cfg, err := config.LoadConfigTOML(configPath)
	if err != nil {
		logrus.Fatalf("Error loading config: %v", err)
	}

	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter admin username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		logrus.Fatalf("Error reading username: %v", err)
	}
	username = strings.TrimSpace(username)

	if username == "" {
		logrus.Fatal("Username cannot be empty")
	}

	fmt.Print("Enter admin password: ")
	passwordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		logrus.Fatalf("Error reading password: %v", err)
	}
	password := string(passwordBytes)
	fmt.Println()

	if len(password) < 4 {
		logrus.Fatal("Password must be at least 4 characters long")
	}

	fmt.Print("Confirm admin password: ")
	confirmPasswordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		logrus.Fatalf("Error reading password confirmation: %v", err)
	}
	confirmPassword := string(confirmPasswordBytes)
	fmt.Println()

	if password != confirmPassword {
		logrus.Fatal("Passwords do not match")
	}

	// Initialize content system for database access
	siteContent := contentstuff.NewContentStuff(&cfg)
	err = siteContent.LoadContent()
	if err != nil {
		logrus.Fatalf("Failed to load content system: %v", err)
	}

	authzApp := &authz.AuthzApp{
		SiteContent: siteContent,
	}

	// Initialize database (migrate tables)
	authzApp.Init()

	// Check if user already exists
	var existingUser authz.User
	err = siteContent.DBHandle.Where("username = ?", username).First(&existingUser).Error

	if err == nil {
		// User exists, update password
		err = authzApp.ChangePassword(existingUser.ID, password)
		if err != nil {
			logrus.Fatalf("Failed to update password: %v", err)
		}
		fmt.Printf("Successfully updated password for admin user '%s'\n", username)
	} else {
		// User doesn't exist, create new admin
		hash, err := authzApp.HashPassword(password)
		if err != nil {
			logrus.Fatalf("Failed to hash password: %v", err)
		}

		user := authz.User{
			Username:     username,
			Email:        fmt.Sprintf("%s@localhost", username),
			PasswordHash: hash,
			Role:         "admin",
		}

		err = siteContent.DBHandle.Create(&user).Error
		if err != nil {
			logrus.Fatalf("Failed to create admin user: %v", err)
		}
		logrus.Infof("Successfully created admin user '%s'", username)
	}
}
