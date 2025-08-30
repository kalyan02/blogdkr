package sitesrv

import (
	"fmt"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/feeds"

	"oddity/pkg/contentstuff"
)

// s.renderRSSFeed(c, requestPath)
func (s *SiteApp) renderRSSFeed(c *gin.Context, requestPath string) {
	if !strings.HasSuffix(requestPath, ".xml") && !strings.HasSuffix(requestPath, ".rss") && !strings.HasSuffix(requestPath, ".atom") {
		s.render404(c)
		return
	}

	// use wire controller to check if this page has any queries
	pathBase := strings.TrimSuffix(requestPath, filepath.Ext(requestPath))

	// check if pathBase exists in site content
	fd, ok := s.SiteContent.DoPath(pathBase)
	if !ok {
		s.render404(c)
		return
	}

	if !s.WireController.PostHasQueries(fd.FileName) {
		s.render404(c)
		return
	}

	posts, err := s.WireController.GetQueryResultsForPost(fd.FileName)
	if err != nil {
		s.renderError(c, requestPath)
		return
	}

	_ = posts

	// get domain from http host headers
	scheme := "http"
	if c.Request.TLS != nil {
		scheme = "https"
	}
	host := scheme + "://" + c.Request.Host

	lastCreated := time.Now()
	if len(posts) > 0 {
		pg := contentstuff.NewPageFromFileDetail(&posts[0])
		if m := pg.DateCreated(); m != nil {
			lastCreated = *m
		}
	}

	now := time.Now()
	feed := &feeds.Feed{
		Title:       s.Config.Site.Title,
		Link:        &feeds.Link{Href: host},
		Description: s.Config.Site.Description,
		Author:      &feeds.Author{Name: s.Config.Site.Title, Email: "jmoiron@jmoiron.net"},
		Created:     lastCreated,
	}

	for _, post := range posts {
		pg := contentstuff.NewPageFromFileDetail(&post)
		if pg.IsPrivate() {
			continue
		}

		item := &feeds.Item{
			Title:       pg.Title(),
			Link:        &feeds.Link{Href: host + "/" + pg.Slug()},
			Description: string(pg.SafeHTML()),
		}

		item.Content = string(pg.SafeHTML())

		if m := pg.DateCreated(); m != nil {
			item.Created = *m
		} else {
			item.Created = now
		}

		if m := pg.DateModified(); m != nil {
			item.Updated = *m
		} else {
			item.Updated = item.Created
		}

		feed.Add(item)

		if len(feed.Items) >= 20 {
			break
		}
	}

	if strings.HasSuffix(requestPath, ".atom") {
		atom, err := feed.ToAtom()
		if err != nil {
			c.Status(http.StatusInternalServerError)
		}
		c.Status(http.StatusOK)
		c.Writer.Header().Set("Content-Type", "application/atom+xml; charset=utf-8")
		_, _ = fmt.Fprintf(c.Writer, atom)
		return
	}
	if strings.HasSuffix(requestPath, ".xml") || strings.HasSuffix(requestPath, ".rss") {
		rss, err := feed.ToRss()
		if err != nil {
			c.Status(http.StatusInternalServerError)
		}
		c.Status(http.StatusOK)
		c.Writer.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		_, _ = fmt.Fprintf(c.Writer, rss)
		return
	}

}
