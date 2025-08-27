package server

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"blogsync2/pkg/config"
	"blogsync2/pkg/dropbox"
	"blogsync2/pkg/sync"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type Server struct {
	config      *config.Config
	syncManager *sync.Manager
	auth        *dropbox.Auth
	client      *dropbox.Client
}

func New(cfg *config.Config, syncManager *sync.Manager, auth *dropbox.Auth) *Server {
	return &Server{
		config:      cfg,
		syncManager: syncManager,
		auth:        auth,
		client:      dropbox.NewClient(auth),
	}
}

func (s *Server) Start(syncChan chan<- sync.Event) error {
	// Configure Gin
	gin.SetMode(gin.ReleaseMode)

	// Public server
	publicRouter := gin.New()
	publicRouter.Use(gin.Logger(), gin.Recovery())
	publicRouter.Use(cors.Default())

	// Routes
	publicRouter.GET("/", s.indexHandler)
	publicRouter.GET(s.config.Server.WebhookPath, s.webhookVerificationHandler)
	publicRouter.POST(s.config.Server.WebhookPath, s.webhookNotificationHandler(syncChan))
	publicRouter.GET("/auth/callback", s.authCallbackHandler)
	publicRouter.GET("/health", s.healthCheckHandler)

	// Admin server
	adminRouter := gin.New()
	adminRouter.Use(gin.Logger(), gin.Recovery())
	adminRouter.Use(cors.Default())

	adminRouter.POST("/admin/sync", s.manualSyncHandler(syncChan))
	adminRouter.POST("/admin/sync_zip", s.syncZipHandler)
	adminRouter.GET("/admin/status", s.adminStatusHandler)
	adminRouter.GET("/admin/auth", s.startAuthHandler)
	adminRouter.GET("/admin/test", s.testDropboxHandler)
	adminRouter.GET("/admin/webhooks", s.webhookHistoryHandler)
	adminRouter.GET("/admin/health", s.healthCheckHandler)

	// Start servers
	go func() {
		addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.Port)
		log.Printf("Starting public server on %s", addr)
		if err := publicRouter.Run(addr); err != nil {
			log.Fatalf("Public server failed: %v", err)
		}
	}()

	go func() {
		addr := fmt.Sprintf("%s:%d", s.config.Server.Host, s.config.Server.AdminPort)
		log.Printf("Starting admin server on %s", addr)
		if err := adminRouter.Run(addr); err != nil {
			log.Fatalf("Admin server failed: %v", err)
		}
	}()

	return nil
}

func (s *Server) indexHandler(c *gin.Context) {
	html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title>BlogSync Service</title>
    <style>
        body { 
            font-family: Arial, sans-serif; 
            text-align: center; 
            margin-top: 100px; 
            background: #f5f5f5; 
        }
        .container { 
            max-width: 600px; 
            margin: 0 auto; 
            padding: 40px; 
            background: white; 
            border-radius: 8px; 
            box-shadow: 0 2px 10px rgba(0,0,0,0.1); 
        }
        .status { color: #4CAF50; font-size: 24px; margin-bottom: 20px; }
        .info { color: #666; margin: 10px 0; }
        .endpoint { 
            background: #f0f0f0; 
            padding: 8px 12px; 
            border-radius: 4px; 
            font-family: monospace; 
            margin: 5px 0; 
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="status">✅ BlogSync Service Running</div>
        <p class="info">Your Dropbox synchronization service is online and ready!</p>
        
        <h3>Public Endpoints</h3>
        <div class="endpoint">GET /health</div>
        <div class="endpoint">GET/POST %s</div>
        <div class="endpoint">GET /auth/callback</div>
        
        <p class="info">
            <strong>Admin endpoints are available on port %d</strong><br>
            (should be firewalled from public access)<br><br>
            <strong>Admin Endpoints:</strong><br>
            <div class="endpoint">POST /admin/sync</div>
            <div class="endpoint">POST /admin/sync_zip</div>
            <div class="endpoint">GET /admin/status</div>
            <div class="endpoint">GET /admin/auth</div>
            <div class="endpoint">GET /admin/test</div>
        </p>
    </div>
</body>
</html>`, s.config.Server.WebhookPath, s.config.Server.AdminPort)

	c.Data(http.StatusOK, "text/html", []byte(html))
}

func (s *Server) webhookVerificationHandler(c *gin.Context) {
	var verification dropbox.WebhookVerification
	if err := c.ShouldBindQuery(&verification); err != nil {
		log.Printf("Webhook verification request missing challenge parameter")
		c.Status(http.StatusBadRequest)
		return
	}

	log.Println("Webhook verification request received")
	c.String(http.StatusOK, verification.Challenge)
}

func (s *Server) webhookNotificationHandler(syncChan chan<- sync.Event) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("=== WEBHOOK RECEIVED ===")
		log.Printf("Timestamp: %s", time.Now().Format(time.RFC3339))

		// Log headers
		for name, values := range c.Request.Header {
			for _, value := range values {
				log.Printf("Header %s: %s", name, value)
			}
		}

		// Read and log raw body
		body, err := c.GetRawData()
		if err != nil {
			log.Printf("Failed to read request body: %v", err)
			c.Status(http.StatusBadRequest)
			return
		}

		log.Printf("Raw body: %s", string(body))

		// Try to parse as JSON if not empty
		var payload dropbox.WebhookNotification
		if len(body) > 0 {
			if err := json.Unmarshal(body, &payload); err != nil {
				log.Printf("Failed to parse JSON payload: %v", err)
			} else {
				log.Printf("Parsed JSON payload: %+v", payload)
			}
		}

		log.Println("========================")

		// Send sync event
		select {
		case syncChan <- sync.Event{Type: sync.FilesChanged, Data: &payload}:
			log.Println("Sync event sent successfully")
			c.String(http.StatusOK, "OK")
		default:
			log.Println("Failed to send sync event: channel full")
			c.Status(http.StatusInternalServerError)
		}
	}
}

func (s *Server) authCallbackHandler(c *gin.Context) {
	log.Println("OAuth callback received")

	if errorParam := c.Query("error"); errorParam != "" {
		log.Printf("OAuth error: %s", errorParam)
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Authentication Failed</title>
<style>body{font-family:Arial;text-align:center;margin-top:100px;}.error{color:#f44336;font-size:24px;}</style>
</head>
<body>
    <div class="error">❌ Authentication Failed</div>
    <p>Dropbox Error: %s</p>
    <p>Please try again.</p>
</body>
</html>`, errorParam)
		c.Data(http.StatusOK, "text/html", []byte(html))
		return
	}

	code := c.Query("code")
	if code == "" {
		log.Println("OAuth callback missing required parameters")
		c.Status(http.StatusBadRequest)
		return
	}

	log.Printf("Received authorization code: %s, exchanging for token", code[:min(10, len(code))])

	if err := s.auth.ExchangeCodeForToken(code); err != nil {
		log.Printf("Token exchange failed: %v", err)
		html := fmt.Sprintf(`<!DOCTYPE html>
<html>
<head><title>Authentication Failed</title>
<style>body{font-family:Arial;text-align:center;margin-top:100px;}.error{color:#f44336;font-size:24px;}</style>
</head>
<body>
    <div class="error">❌ Authentication Failed</div>
    <p>Error: %s</p>
    <p>Please try again or check the logs.</p>
</body>
</html>`, err.Error())
		c.Data(http.StatusOK, "text/html", []byte(html))
		return
	}

	log.Println("Token exchange successful!")
	html := `<!DOCTYPE html>
<html>
<head><title>Authentication Successful</title>
<style>body{font-family:Arial;text-align:center;margin-top:100px;}.success{color:#4CAF50;font-size:24px;}</style>
</head>
<body>
    <div class="success">✅ Authentication Successful!</div>
    <p>Your Dropbox account has been successfully linked.</p>
    <p>You can now close this window and use the BlogSync service.</p>
    <script>setTimeout(() => window.close(), 3000);</script>
</body>
</html>`
	c.Data(http.StatusOK, "text/html", []byte(html))
}

func (s *Server) healthCheckHandler(c *gin.Context) {
	respondJSON(c, http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
	})
}

func (s *Server) manualSyncHandler(syncChan chan<- sync.Event) gin.HandlerFunc {
	return func(c *gin.Context) {
		log.Println("Manual sync requested")

		select {
		case syncChan <- sync.Event{Type: sync.ForceSync}:
			respondJSON(c, http.StatusOK, gin.H{
				"status":    "sync_triggered",
				"message":   "Sync process has been triggered",
				"timestamp": time.Now().Format(time.RFC3339),
			})
		default:
			log.Println("Failed to send manual sync event: channel full")
			c.Status(http.StatusInternalServerError)
		}
	}
}

func (s *Server) syncZipHandler(c *gin.Context) {
	log.Println("Zip sync requested")

	if !s.auth.HasValidToken() {
		respondJSON(c, http.StatusOK, gin.H{
			"status":  "error",
			"message": "Not authenticated. Run /admin/auth first.",
		})
		return
	}

	tempZipPath := "/tmp/dropbox_sync.zip"

	log.Printf("Requesting download from path: %s", s.config.Sync.DropboxFolder)
	log.Printf("Temporary zip path: %s", tempZipPath)

	if err := s.client.DownloadZip(s.config.Sync.DropboxFolder, tempZipPath); err != nil {
		log.Printf("Zip download failed: %v", err)
		respondJSON(c, http.StatusOK, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Zip download failed: %v", err),
		})
		return
	}

	// Check if file was created and get its size
	stat, err := os.Stat(tempZipPath)
	if err != nil {
		log.Printf("Failed to get zip metadata: %v", err)
		respondJSON(c, http.StatusOK, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Failed to get zip metadata: %v", err),
		})
		return
	}

	log.Printf("Successfully downloaded zip file: %d bytes", stat.Size())
	zipSize := stat.Size()

	// Extract the zip contents to sync folder
	syncFolder := s.config.Sync.LocalBasePath
	extractedCount, err := s.syncManager.ExtractZip(tempZipPath, syncFolder)
	if err != nil {
		log.Printf("Failed to extract zip: %v", err)
		// Clean up the temp file
		os.Remove(tempZipPath)
		respondJSON(c, http.StatusOK, gin.H{
			"status":   "error",
			"message":  fmt.Sprintf("Downloaded zip but failed to extract: %v", err),
			"zip_size": zipSize,
		})
		return
	}

	log.Printf("Successfully extracted %d files from zip", extractedCount)

	respondJSON(c, http.StatusOK, gin.H{
		"status":          "success",
		"message":         "Successfully downloaded and extracted zip from Dropbox",
		"zip_size":        zipSize,
		"files_extracted": extractedCount,
		"extracted_to":    syncFolder,
		"timestamp":       time.Now().Format(time.RFC3339),
	})
}

func (s *Server) adminStatusHandler(c *gin.Context) {
	hasValidToken := s.auth.HasValidToken()

	response := gin.H{
		"status":        "running",
		"timestamp":     time.Now().Format(time.RFC3339),
		"authenticated": hasValidToken,
		"config": gin.H{
			"public_port":   s.config.Server.Port,
			"admin_port":    s.config.Server.AdminPort,
			"webhook_path":  s.config.Server.WebhookPath,
			"sync_path":     s.config.Sync.LocalBasePath,
			"build_command": s.config.Build.Command,
		},
	}

	if hasValidToken {
		userInfo, err := s.client.GetCurrentAccount()
		if err != nil {
			log.Printf("Failed to get user info: %v", err)
			response["dropbox_user"] = gin.H{
				"error": fmt.Sprintf("Failed to get user info: %v", err),
			}
		} else {
			response["dropbox_user"] = gin.H{
				"display_name": userInfo.Name.DisplayName,
				"email":        userInfo.Email,
				"account_id":   userInfo.AccountID,
			}
		}
	} else {
		response["dropbox_user"] = nil
	}

	respondJSON(c, http.StatusOK, response)
}

func (s *Server) webhookHistoryHandler(c *gin.Context) {
	respondJSON(c, http.StatusOK, gin.H{
		"message": "Webhook history not implemented yet - check service logs for webhook activity",
		"tip":     "Look for '=== WEBHOOK RECEIVED ===' in logs",
	})
}

func (s *Server) startAuthHandler(c *gin.Context) {
	log.Println("Auth flow requested via admin endpoint")

	if s.auth.HasValidToken() {
		respondJSON(c, http.StatusOK, gin.H{
			"status":  "already_authenticated",
			"message": "Already authenticated with valid token",
		})
		return
	}

	state := uuid.New().String()
	authURL, err := s.auth.GetAuthorizationURL(state)
	if err != nil {
		log.Printf("Failed to generate auth URL: %v", err)
		c.Status(http.StatusInternalServerError)
		return
	}

	log.Printf("Generated auth URL: %s", authURL)
	respondJSON(c, http.StatusOK, gin.H{
		"status":       "auth_url_generated",
		"auth_url":     authURL,
		"message":      "Open this URL in your browser to authenticate",
		"callback_url": fmt.Sprintf("http://%s:%d/auth/callback", s.config.Server.Host, s.config.Server.Port),
	})
}

func (s *Server) testDropboxHandler(c *gin.Context) {
	log.Println("Testing Dropbox connection")

	if !s.auth.HasValidToken() {
		respondJSON(c, http.StatusOK, gin.H{
			"status":  "error",
			"message": "Not authenticated. Run /admin/auth first.",
		})
		return
	}

	files, _, err := s.client.ListFolder("/", false)
	if err != nil {
		log.Printf("Dropbox API error: %v", err)
		respondJSON(c, http.StatusOK, gin.H{
			"status":  "error",
			"message": fmt.Sprintf("Dropbox API error: %v", err),
		})
		return
	}

	// Take first 10 files for display
	var filePaths []string
	for i, file := range files {
		if i >= 10 {
			break
		}
		filePaths = append(filePaths, file.Path)
	}

	note := "All files shown"
	if len(files) > 10 {
		note = "Showing first 10 files only"
	}

	respondJSON(c, http.StatusOK, gin.H{
		"status":     "success",
		"message":    "Successfully connected to Dropbox",
		"file_count": len(files),
		"files":      filePaths,
		"note":       note,
	})
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// respondJSON sends a formatted JSON response without HTML escaping
func respondJSON(c *gin.Context, statusCode int, obj interface{}) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "    ")

	if err := encoder.Encode(obj); err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}

	c.Data(statusCode, "application/json; charset=utf-8", buf.Bytes())
}
