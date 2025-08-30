package sitesrv

import (
	"fmt"
	"html/template"
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
		for _, staticDir := range s.SiteContent.Config.StaticDirs {
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

func (s *SiteApp) buildPageNavLinks(page *contentstuff.Page) []config.NavigationLink {

	parentSlug := "/"
	if strings.Contains(page.Slug(), "/") {
		parentSlug = filepath.Dir(page.Slug())
		parentSlug = strings.Trim(parentSlug, "./")
		// add prefix
		parentSlug = "/" + parentSlug
	}

	if parentSlug == "/" || parentSlug == "/index" || parentSlug == "/blog" {
		return s.Config.Site.Navigation
	}

	var links []config.NavigationLink

	prevLink := config.NavigationLink{
		Name: "Home",
		URL:  parentSlug,
	}
	links = append(links, prevLink)

	return links
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

	sc := s.Config.Site
	sc.Navigation = s.buildPageNavLinks(page)

	indexPage := contentstuff.PostPage{
		Site: sc,
		Meta: contentstuff.PageMeta{
			Title: page.Title(),
		},
		PageHTML:        page.SafeHTML(),
		NewPostHintSlug: s.createNewPostSlugHint(page),
		EditURL:         fmt.Sprintf("/admin/edit?path=%s", page.Slug()),
		IsPrivate:       page.IsPrivate(),
		IsAuthenticated: authz.IsAuthenticated(c),
		BackLink:        s.backLinkToParent(page.Slug()),
	}

	c.HTML(200, "post.html", indexPage)
	fmt.Println(c.Errors)
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

	sc := s.Config.Site
	sc.Navigation = s.buildPageNavLinks(page)

	postPage := contentstuff.PostPage{
		Site:            sc,
		EditURL:         fmt.Sprintf("/admin/edit?path=%s", page.Slug()),
		IsAuthenticated: authz.IsAuthenticated(c),
		IsPrivate:       contentstuff.IsPrivate(s.SiteContent, file),
		NewPostHintSlug: s.createNewPostSlugHint(page),
		Meta: contentstuff.PageMeta{
			Title: page.Title(),
		},
		PageHTML:    page.SafeHTML(),
		CreatedDate: page.DateCreated(),
		BackLink:    s.backLinkToParent(page.Slug()),
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
		slugDir = s.SiteContent.Config.DefaultNewHint
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

func (s *SiteApp) render404(c *gin.Context) {
	postPage := contentstuff.PostPage{
		Site: s.Config.Site,
	}
	postPage.Meta = contentstuff.PageMeta{
		Title: "404 Not Found",
	}
	postPage.PageHTML = template.HTML("<p>The page you are looking for does not exist.</p>")

	c.HTML(404, "post.html", postPage)

}

func (s *SiteApp) render404ButMaybeCreate(c *gin.Context, path string) {
	postPage := contentstuff.PostPage{
		Site:            s.Config.Site,
		IsAuthenticated: authz.IsAuthenticated(c),
		NewPostHintSlug: s.createNewPostSlugHintFromPath(path),
	}
	postPage.Meta = contentstuff.PageMeta{
		Title: "404 Not Found",
	}
	postPage.PageHTML = template.HTML(fmt.Sprintf(`
<p>The page you are looking for does not exist.</p>
<p><a href="/admin/edit?path=%s">Create it</a></p>`, path))

	c.HTML(404, "post.html", postPage)

}
