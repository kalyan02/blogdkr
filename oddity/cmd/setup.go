package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"
	"syscall"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh/terminal"

	"oddity/pkg/authz"
	"oddity/pkg/config"
	"oddity/pkg/contentstuff"
)

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup commands",
	Long:  `Commands for managing admin users and system administration.`,
}

var adminAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Set up admin username and password",
	Long:  `Interactive command to set up or update admin username and password.`,
	Run:   runAdminAuthSetup,
}

var newConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Generate default configuration file",
	Long:  `Generates a default configuration file named config.yaml in the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		defaultConfig := config.NewConfig()
		configData, err := defaultConfig.EncodeTOML()
		if err != nil {
			log.Fatalf("Failed to marshal default config: %v", err)
		}

		configFilePath := "config.toml"
		if _, err := os.Stat(configFilePath); err == nil {
			log.Fatalf("Config file %s already exists. Aborting to prevent overwrite.", configFilePath)
		}

		err = os.WriteFile(configFilePath, configData, 0644)
		if err != nil {
			log.Fatalf("Failed to write config file: %v", err)
		}

		fmt.Printf("Default configuration file created at %s\n", configFilePath)
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.AddCommand(adminAuthCmd)
	rootCmd.AddCommand(newConfigCmd)
}

func runAdminAuthSetup(cmd *cobra.Command, args []string) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Print("Enter admin username: ")
	username, err := reader.ReadString('\n')
	if err != nil {
		log.Fatalf("Error reading username: %v", err)
	}
	username = strings.TrimSpace(username)

	if username == "" {
		log.Fatal("Username cannot be empty")
	}

	fmt.Print("Enter admin password: ")
	passwordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("Error reading password: %v", err)
	}
	password := string(passwordBytes)
	fmt.Println()

	if len(password) < 4 {
		log.Fatal("Password must be at least 4 characters long")
	}

	fmt.Print("Confirm admin password: ")
	confirmPasswordBytes, err := terminal.ReadPassword(int(syscall.Stdin))
	if err != nil {
		log.Fatalf("Error reading password confirmation: %v", err)
	}
	confirmPassword := string(confirmPasswordBytes)
	fmt.Println()

	if password != confirmPassword {
		log.Fatal("Passwords do not match")
	}

	// Initialize content system for database access
	siteContent := contentstuff.NewContentStuff(config.DefaultConfig)
	err = siteContent.LoadContent()
	if err != nil {
		log.Fatalf("Failed to load content system: %v", err)
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
			log.Fatalf("Failed to update password: %v", err)
		}
		fmt.Printf("Successfully updated password for admin user '%s'\n", username)
	} else {
		// User doesn't exist, create new admin
		hash, err := authzApp.HashPassword(password)
		if err != nil {
			log.Fatalf("Failed to hash password: %v", err)
		}

		user := authz.User{
			Username:     username,
			Email:        fmt.Sprintf("%s@localhost", username),
			PasswordHash: hash,
			Role:         "admin",
		}

		err = siteContent.DBHandle.Create(&user).Error
		if err != nil {
			log.Fatalf("Failed to create admin user: %v", err)
		}
		fmt.Printf("Successfully created admin user '%s'\n", username)
	}
}
