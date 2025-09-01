package sitesrv

import (
	"fmt"
	"html/template"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"

	"oddity/pkg/authz"
	"oddity/pkg/config"
	"oddity/pkg/contentstuff"
)

type SiteApp struct {
	WireController *contentstuff.Wire
	SiteContent    *contentstuff.ContentStuff
	Config         config.Config
}

func (s *SiteApp) RegisterRoutes(r *gin.Engine) {
	r.NoRoute(s.handleAllContentPages)
}

func (s *SiteApp) handleAllContentPages(c *gin.Context) {
	requestPath := c.Request.URL.Path

	logrus.Infof("Handling request for path: %s", requestPath)

	// check if it is a static file is that's requested
	if IsStaticFile(requestPath) {
		for _, staticDir := range s.SiteContent.Config.Content.StaticDirs {
			staticFilePath := filepath.Join(staticDir, requestPath)
			if _, err := os.Stat(staticFilePath); err == nil {
				c.File(staticFilePath)
				return
			}
		}
	}

	// trim suffix
	requestPath = strings.TrimPrefix(requestPath, "/")
	requestPath = strings.TrimSuffix(requestPath, "/")

	if requestPath == "" {
		requestPath = "."
	}

	if requestPath == "index.html" || requestPath == "index" {
		// just redirect to root
		c.Redirect(http.StatusFound, "/")
		return
	}

	if strings.HasSuffix(requestPath, ".html") {
		requestPath = strings.TrimSuffix(requestPath, ".html")
		requestPath = strings.Trim(requestPath, "/")

		// check if path without .html exists
		if _, ok := s.SiteContent.DoPath(requestPath); ok {
			// if it exists, redirect to it
			c.Redirect(http.StatusFound, "/"+requestPath)
			return
		}
	}

	if strings.HasSuffix(requestPath, ".xml") || strings.HasSuffix(requestPath, ".rss") || strings.HasSuffix(requestPath, ".atom") {
		// handle rss feed request
		s.renderRSSFeed(c, requestPath)
		return
	}

	if file, ok := s.SiteContent.DoPath(requestPath); ok {
		if file.FileType == contentstuff.FileTypeDirectory {
			// look for index.md or index.html in this directory
			s.renderIndexAtPath(c, requestPath)
			return
		}
		// if ends with index.html or index.md then render index of parent directory
		if strings.HasSuffix(requestPath, "index.html") || strings.HasSuffix(requestPath, "index.md") || strings.HasSuffix(requestPath, "index") {
			s.renderIndexFileAtPath(c, requestPath)
			return
		}

		s.renderPage(c, file)
		return
	}

	if authz.IsAuthenticated(c) {
		s.render404ButMaybeCreate(c, requestPath)
		return
	}

	//c.String(404, "Not Found")
	s.render404(c)
}

func (s *SiteApp) renderIndexAtPath(c *gin.Context, path string) {
	potentialIdxFiles := []string{
		filepath.Join(path, "index.md"),
		filepath.Join(path, "index.html"),
	}
	for _, idxFile := range potentialIdxFiles {
		if _, ok := s.SiteContent.DoPath(idxFile); ok {
			s.renderIndexFileAtPath(c, idxFile)
			return
		}
	}

	if authz.IsAuthenticated(c) {
		// if none of those exist then show 404
		defaultIndexPath := filepath.Join(path, "index")
		s.render404ButMaybeCreate(c, defaultIndexPath)
		return
	}

	s.render404(c)
}

func (s *SiteApp) backLinkToParent(path string) string {
	if path == "" || path == "." || path == "/" || path == "index" {
		return ""
	}
	if strings.Contains(path, "/") {
		parentSlug := filepath.Dir(path)
		parentSlug = strings.Trim(parentSlug, "./")
		// add prefix
		parentSlug = "/" + parentSlug
		return parentSlug
	}
	return "/"
}

func (s *SiteApp) renderIndexFileAtPath(c *gin.Context, path string) {
	file, ok := s.SiteContent.DoPath(path)
	if !ok {
		s.render404(c)
		return
	}
	page := contentstuff.NewPageFromFileDetail(&file)
	if page.IsPrivate() && !authz.IsAuthenticated(c) {
		s.render404(c)
		return
	}

	indexPage := contentstuff.PostPage{
		Site: s.buildSiteConfigWithNav(c, page.Slug()),
		Meta: contentstuff.PageMeta{
			Title: page.Title(),
		},
		PageHTML:        page.SafeHTML(),
		NewPostHintSlug: s.createNewPostSlugHint(page),
		EditURL:         fmt.Sprintf("/admin/edit?path=%s", page.Slug()),
		IsPrivate:       page.IsPrivate(),
		IsAuthenticated: authz.IsAuthenticated(c),
		BackLink:        s.backLinkToParent(page.Slug()),
		FeedsLink:       s.createFeedsLink(page),
	}

	c.HTML(200, "post.html", indexPage)
	fmt.Println(c.Errors)
}

func (s *SiteApp) buildSiteConfigWithNav(c *gin.Context, page string) config.SiteConfig {
	isAuth := authz.IsAuthenticated(c)
	sc := s.Config.GetSiteConfig(isAuth)

	parentSlug := "/"
	if strings.Contains(page, "/") {
		parentSlug = filepath.Dir(page)
		parentSlug = strings.Trim(parentSlug, "./")
		// add prefix
		parentSlug = "/" + parentSlug
	}

	if parentSlug == "/" || parentSlug == "/index" || parentSlug == "/blog" {
		return sc
	}

	var links []config.NavigationLink

	prevLink := config.NavigationLink{
		Name: "Home",
		URL:  parentSlug,
	}
	links = append(links, prevLink)

	sc.Navigation = links

	return sc
}

func (s *SiteApp) renderPage(c *gin.Context, file contentstuff.FileDetail) {
	// load the file content and render it
	page := contentstuff.NewPageFromFileDetail(&file)

	if !authz.IsAuthenticated(c) {
		if contentstuff.IsPrivate(s.SiteContent, file) {
			s.render404(c)
			return
		}
	}

	postPage := contentstuff.PostPage{
		Site:            s.buildSiteConfigWithNav(c, page.Slug()),
		EditURL:         fmt.Sprintf("/admin/edit?path=%s", page.Slug()),
		IsAuthenticated: authz.IsAuthenticated(c),
		IsPrivate:       contentstuff.IsPrivate(s.SiteContent, file),
		NewPostHintSlug: s.createNewPostSlugHint(page),
		Meta: contentstuff.PageMeta{
			Title: page.Title(),
		},
		PageHTML:     page.SafeHTML(),
		CreatedDate:  page.DateCreated(),
		ModifiedDate: page.DateModified(),
		BackLink:     s.backLinkToParent(page.Slug()),
		FeedsLink:    s.createFeedsLink(page),
	}
	//postPage.ModifiedDate = p.DateModified()

	c.HTML(200, "post.html", postPage)
}

func (s *SiteApp) createNewPostSlugHint(path *contentstuff.Page) string {
	currSlug := path.Slug()
	return s.createNewPostSlugHintFromPath(currSlug)
}

func (s *SiteApp) createNewPostSlugHintFromPath(currSlug string) string {
	slugDir := filepath.Dir(currSlug)
	if slugDir == "." {
		slugDir = s.SiteContent.Config.GetSiteConfig(true).DefaultNewHint
	}

	today := time.Now().Format("2006-01-02")
	hintSlug := filepath.Join(slugDir, today)
	i := 1
	for {
		if _, ok := s.SiteContent.SlugFileMap[hintSlug]; !ok {
			break
		}
		i++
		hintSlug = filepath.Join(slugDir, fmt.Sprintf("%s-%d", today, i))
	}
	return hintSlug
}

func (s *SiteApp) createFeedsLink(pg *contentstuff.Page) string {
	if pg == nil {
		return ""
	}
	slug := pg.Slug()
	if s.WireController.PostHasQueries(pg.File.FileName) {
		return fmt.Sprintf("/%s.xml", slug)
	}
	return ""
}

func (s *SiteApp) render404(c *gin.Context) {
	postPage := contentstuff.PostPage{
		Site: s.buildSiteConfigWithNav(c, ""), // page is only used for nav and we don't care for public 404
	}
	postPage.Meta = contentstuff.PageMeta{
		Title: "404 Not Found",
	}
	postPage.PageHTML = template.HTML("<p>The page you are looking for does not exist.</p>")

	c.HTML(http.StatusNotFound, "post.html", postPage)

}

func (s *SiteApp) render404ButMaybeCreate(c *gin.Context, path string) {
	postPage := contentstuff.PostPage{
		Site:            s.buildSiteConfigWithNav(c, path),
		IsAuthenticated: authz.IsAuthenticated(c),
		NewPostHintSlug: s.createNewPostSlugHintFromPath(path),
	}
	postPage.Meta = contentstuff.PageMeta{
		Title: "404 Not Found",
	}

	replacer := strings.NewReplacer(
		"{path}", path,
	)

	postPage.PageHTML = template.HTML(replacer.Replace(`
<p>The page you are looking for does not exist.</p>
<p>Create as<br>
 - <a href="/admin/edit?path={path}">page: ({path}.md)</a><br>
 - <a href="/admin/edit?path={path}/index">folder: ({path}/index.md)</a><br>
</p>
`))

	c.HTML(http.StatusNotFound, "post.html", postPage)

}

func (s *SiteApp) renderError(c *gin.Context, path string) {
	postPage := contentstuff.PostPage{
		Site:            s.buildSiteConfigWithNav(c, path),
		IsAuthenticated: authz.IsAuthenticated(c),
		NewPostHintSlug: s.createNewPostSlugHintFromPath(path),
		Meta: contentstuff.PageMeta{
			Title: "Error",
		},
		PageHTML: template.HTML(fmt.Sprintf(`<p>There was an error processing your request for %s</p>`, path)),
	}
	c.HTML(http.StatusInternalServerError, "post.html", postPage)
}
