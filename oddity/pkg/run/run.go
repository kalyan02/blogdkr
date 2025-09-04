package run

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"oddity/pkg/admin"
	"oddity/pkg/authz"
	"oddity/pkg/config"
	"oddity/pkg/contentstuff"
	"oddity/pkg/sitesrv"
)

var siteContent *contentstuff.ContentStuff
var wireController *contentstuff.Wire

func StartServer(cfg config.Config) {
	startT := time.Now()
	siteContent = contentstuff.NewContentStuff(&cfg)
	err := siteContent.LoadContent()
	if err != nil {
		logrus.Fatalf("error loading content: %v", err)
	}
	logrus.Infof("Loaded %d content files in %v", len(siteContent.AllFiles()), time.Since(startT))

	//closeCh, err := siteContent.WatchContentChanges()
	//if err != nil {
	//	log.Fatalf("error watching content changes: %v", err)
	//}
	//defer func() {
	//	close(closeCh)
	//}()

	startT = time.Now()
	wireController = contentstuff.NewWire(siteContent)
	err = wireController.ScanForQueries()
	if err != nil {
		logrus.Fatalf("error scanning for queries: %v", err)
	}
	logrus.Infof("Scanned %d query files in %v", wireController.QueryCount(), time.Since(startT))

	// notify all index files
	allFiles := siteContent.AllFiles()
	for _, fd := range allFiles {
		fname := fd.FileName

		if strings.HasSuffix(fname, "index.md") || strings.HasSuffix(fname, "index.html") {
			err = wireController.NotifyFileChanged(fname)
			if err != nil {
				logrus.Errorf("error notifying file changed: %v", err)
			}
			fmt.Println("Index file:", fname)

			err = siteContent.RefreshContent(fname)
			if err != nil {
				logrus.Errorf("error refreshing content for %s: %v", fname, err)
			}
		}
	}

	r := gin.Default()
	tmplDir := cfg.Content.ThemeDir
	if tmplDir == "" {
		tmplDir = "tmpl"
	}
	r.LoadHTMLGlob(filepath.Join(tmplDir, "*.html"))

	// serve static files from uploadsdir at /uploads
	uploadsDir := cfg.Content.UploadDir
	if uploadsDir != "" {
		r.Static("/uploads", uploadsDir)
		logrus.Infof("Serving static files from %s at /uploads", uploadsDir)
	} else {
		logrus.Warn("UploadsDir is not set in config, static files will not be served")
	}

	siteApp := &sitesrv.SiteApp{
		Config:         cfg,
		SiteContent:    siteContent,
		WireController: wireController,
	}

	authzApp := &authz.AuthzApp{
		SiteContent: siteContent,
	}
	authzApp.Init()

	adminApp := &admin.AdminApp{
		SiteContent:    siteContent,
		WireController: wireController,
		Authz:          authzApp,
	}

	// inspect cloudflare headers middleware
	// oddity  | time="2025-09-02T10:33:23Z" level=info msg="Cf-Visitor: {\"scheme\":\"https\"}"
	// if the cf visitor header is set, and if scheme is not https, redirect to https path
	r.Use(func(c *gin.Context) {
		cfVisitor := c.GetHeader("Cf-Visitor")
		if cfVisitor != "" && strings.Contains(cfVisitor, "\"scheme\":\"http\"") {
			newURL := "https://" + c.Request.Host + c.Request.RequestURI
			logrus.Infof("Redirecting to HTTPS: %s", newURL)
			c.Redirect(http.StatusFound, newURL)
			c.Abort()
			return
		}
		c.Next()
	})

	
	// auth middleware
	r.Use(authzApp.AuthMiddleware())

	adminApp.RegisterRoutes(r)

	siteApp.RegisterRoutes(r)
	authzApp.RegisterRoutes(r)

	r.Run(cfg.Content.Addr) // listen and serve
}
