package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"blogsync2/pkg/config"
	"blogsync2/pkg/db"
	"blogsync2/pkg/dropbox"
	"blogsync2/pkg/server"
	"blogsync2/pkg/sync"
	"blogsync2/pkg/token"
)

func main() {
	var (
		configFile = flag.String("config", "config.toml", "Configuration file path")
		password   = flag.String("password", "", "Password for token encryption")
		command    = flag.String("cmd", "start", "Command to run: start, init-config, token")
	)
	flag.Parse()

	switch *command {
	case "init-config":
		if err := generateDefaultConfig(*configFile); err != nil {
			log.Fatalf("Failed to generate config: %v", err)
		}
		fmt.Printf("Generated default configuration at: %s\n", *configFile)

	case "start":
		if err := startServer(*configFile, *password); err != nil {
			log.Fatalf("Server failed: %v", err)
		}

	case "token":
		if err := printToken(*configFile, *password); err != nil {
			log.Fatalf("Failed to get token: %v", err)
		}

	default:
		fmt.Printf("Unknown command: %s\n", *command)
		os.Exit(1)
	}
}

func generateDefaultConfig(configPath string) error {
	cfg := config.Default()
	return config.Save(cfg, configPath)
}

func startServer(configFile, password string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	pwd := getPassword(password)

	// Connect to database
	database, err := db.DBConnect(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	tokenStorage, err := token.NewSecureStorage(token.GetDefaultTokenPath(), pwd)
	if err != nil {
		return fmt.Errorf("failed to create token storage: %w", err)
	}

	// Database is ready for use
	_ = database

	auth := dropbox.NewAuth(cfg.Dropbox, tokenStorage)
	client := dropbox.NewClient(auth)

	syncManager := sync.NewManager(cfg, client, database)
	webServer := server.New(cfg, syncManager, auth)

	// Start sync manager in background
	syncChan := make(chan sync.Event, 100)
	go syncManager.Start(syncChan)

	// Start web server
	go webServer.Start(syncChan)

	log.Printf("BlogSync service started")
	log.Printf("Public server: http://%s:%d (webhooks, auth)", cfg.Server.Host, cfg.Server.Port)
	log.Printf("Admin server: http://%s:%d (admin endpoints)", cfg.Server.Host, cfg.Server.AdminPort)
	log.Printf("Webhook endpoint: http://%s:%d%s", cfg.Server.Host, cfg.Server.Port, cfg.Server.WebhookPath)
	log.Printf("Auth callback: http://%s:%d/auth/callback", cfg.Server.Host, cfg.Server.Port)

	// Wait for shutdown signal
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	<-c

	log.Println("Shutdown signal received")
	return nil
}

func printToken(configFile, password string) error {
	cfg, err := config.Load(configFile)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	pwd := getPassword(password)

	tokenStorage, err := token.NewSecureStorage(token.GetDefaultTokenPath(), pwd)
	if err != nil {
		return fmt.Errorf("failed to create token storage: %w", err)
	}

	auth := dropbox.NewAuth(cfg.Dropbox, tokenStorage)

	accessToken, err := auth.GetValidAccessToken()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get access token: %v\n", err)
		fmt.Fprintf(os.Stderr, "You may need to authenticate first by running the server and visiting /admin/auth\n")
		os.Exit(1)
	}

	fmt.Println(accessToken)
	return nil
}

func getPassword(password string) string {
	if password != "" {
		return password
	}

	if envPassword := os.Getenv("DROPBOX_SYNC_PASSWORD"); envPassword != "" {
		return envPassword
	}

	return ""

	fmt.Print("Enter password for token encryption: ")
	var pwd string
	fmt.Scanln(&pwd)
	return pwd
}
