package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	log "github.com/sirupsen/logrus"

	"oddity/pkg/admin"
	"oddity/pkg/authz"
	"oddity/pkg/config"
	"oddity/pkg/contentstuff"
	"oddity/pkg/sitesrv"
)

var siteContent *contentstuff.ContentStuff
var wireController *contentstuff.Wire

func main() {

	startT := time.Now()
	siteContent = contentstuff.NewContentStuff(config.DefaultConfig)
	err := siteContent.LoadContent()
	if err != nil {
		log.Fatalf("error loading content: %v", err)
	}
	log.Infof("Loaded %d content files in %v", len(siteContent.AllFiles()), time.Since(startT))

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
		log.Fatalf("error scanning for queries: %v", err)
	}
	log.Infof("Scanned %d query files in %v", wireController.QueryCount(), time.Since(startT))

	// notify all index files
	allFiles := siteContent.AllFiles()
	for _, fd := range allFiles {
		fname := fd.FileName

		if strings.HasSuffix(fname, "index.md") || strings.HasSuffix(fname, "index.html") {
			err = wireController.NotifyFileChanged(fname)
			if err != nil {
				log.Errorf("error notifying file changed: %v", err)
			}
			fmt.Println("Index file:", fname)

			err = siteContent.RefreshContent(fname)
			if err != nil {
				log.Errorf("error refreshing content for %s: %v", fname, err)
			}
		}
	}

	r := gin.Default()
	r.LoadHTMLGlob("tmpl/*")

	siteApp := &sitesrv.SiteApp{
		SiteContent:    siteContent,
		WireController: wireController,
	}

	adminApp := &admin.AdminApp{
		SiteContent:    siteContent,
		WireController: wireController,
	}

	authzApp := &authz.AuthzApp{
		SiteContent:    siteContent,
		WireController: wireController,
	}
	authzApp.Init()

	// auth middleware
	r.Use(authzApp.AuthMiddleware())

	adminGroup := r.Group("/admin")
	adminGroup.Use(authzApp.RequireAuth())
	adminGroup.GET("/edit", adminApp.HandleAdminEditor)
	adminGroup.Any("/edit-data", adminApp.HandleEditPageData)

	siteApp.RegisterRoutes(r)
	authzApp.RegisterRoutes(r)

	r.Run(":8081")
}
