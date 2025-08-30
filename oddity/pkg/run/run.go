package run

import (
	"fmt"
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
	siteContent = contentstuff.NewContentStuff(cfg.Content)
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
	r.LoadHTMLGlob("tmpl/*")

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
