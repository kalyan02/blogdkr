package main

import (
	"html/template"
	"time"
)

// NavigationLink represents a navigation menu item
type NavigationLink struct {
	Name       string `json:"name"`
	URL        string `json:"url"`
	IsActive   bool   `json:"is_active"`
	IsExternal bool   `json:"is_external,omitempty"`
}

// SiteConfig holds common site-wide configuration and navigation
type SiteConfig struct {
	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	BaseURL     string           `json:"base_url,omitempty"`
	Navigation  []NavigationLink `json:"navigation"`
}

// PageMeta holds common metadata for all page types
type PageMeta struct {
	Title        string    `json:"title"`
	Description  string    `json:"description,omitempty"`
	Keywords     []string  `json:"keywords,omitempty"`
	Author       string    `json:"author,omitempty"`
	PublishDate  time.Time `json:"publish_date,omitempty"`
	ModifyDate   time.Time `json:"modify_date,omitempty"`
	CanonicalURL string    `json:"canonical_url,omitempty"`
}

// PostPage represents the data structure for rendering individual posts/pages
type PostPage struct {
	// Common page data
	Site SiteConfig `json:"site"`
	Meta PageMeta   `json:"meta"`

	// Page-specific metadata
	CreatedDate  *time.Time `json:"created_date"`
	ModifiedDate *time.Time `json:"modified_date"`
	ReadingTime  int        `json:"reading_time"` // in minutes
	Tags         []string   `json:"tags"`
	Slug         string     `json:"slug"`
	EditURL      string     `json:"edit_url,omitempty"`

	// Content
	PageHTML template.HTML `json:"page_html"`

	// Wiki-like features
	Backlinks       []WikiLink `json:"backlinks,omitempty"`
	LinkedPages     []WikiLink `json:"linked_pages,omitempty"`
	WordCount       int        `json:"word_count,omitempty"`
	NewPostHintSlug string     `json:"new_post_hint_slug,omitempty"`
	AdminLogged     bool       `json:"admin_logged,omitempty"`
	ParentSlug      string     `json:"parent_slug,omitempty"`
}

// IndexPage represents the data structure for rendering the main blog index
type IndexPage struct {
	// Common page data
	Site SiteConfig `json:"site"`
	Meta PageMeta   `json:"meta"`

	// Content
	PageHTML template.HTML `json:"page_html"`
	Posts    []PostSummary `json:"posts"`
}

// PostSummary represents a blog post in list/summary format
type PostSummary struct {
	Title   string    `json:"title"`
	Slug    string    `json:"slug"`
	Summary string    `json:"summary,omitempty"`
	Date    time.Time `json:"date"`
	Tags    []string  `json:"tags,omitempty"`
}

// PostsPage represents the data structure for rendering the posts listing page
type PostsPage struct {
	// Common page data
	Site SiteConfig `json:"site"`
	Meta PageMeta   `json:"meta"`

	// Page-specific metadata
	PostCount int `json:"post_count,omitempty"`

	// Content organization
	PostsByYear []YearGroup `json:"posts_by_year"`
	AllTags     []TagInfo   `json:"all_tags,omitempty"`
	Archives    []Archive   `json:"archives,omitempty"`
}

// YearGroup represents posts grouped by year
type YearGroup struct {
	Year  int           `json:"year"`
	Posts []PostSummary `json:"posts"`
}

// TagInfo represents tag information with post count
type TagInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// Archive represents yearly archive information
type Archive struct {
	Year  int `json:"year"`
	Count int `json:"count"`
}

// WikiLink represents a wiki-style link between pages
type WikiLink struct {
	Title   string `json:"title"`
	Slug    string `json:"slug"`
	Excerpt string `json:"excerpt,omitempty"`
}

// Constructor functions for easy creation

// NewSiteConfig creates a new SiteConfig with default navigation
func NewSiteConfig(title string) *SiteConfig {
	return &SiteConfig{
		Title: title,
		Navigation: []NavigationLink{
			{Name: "Home", URL: "/", IsActive: false},
			{Name: "About", URL: "/about", IsActive: false},
			{Name: "Posts", URL: "/posts", IsActive: false},
		},
	}
}

// NewPostPage creates a new PostPage with default values
func NewPostPage(title string, site *SiteConfig) *PostPage {
	if site == nil {
		site = NewSiteConfig("Wiki")
	}

	return &PostPage{
		Site: *site,
		Meta: PageMeta{
			Title: title,
		},
		CreatedDate:  nil,
		ModifiedDate: nil,
		Tags:         make([]string, 0),
		Backlinks:    make([]WikiLink, 0),
		LinkedPages:  make([]WikiLink, 0),
	}
}

// NewIndexPage creates a new IndexPage with default values
func NewIndexPage(title string, site *SiteConfig) *IndexPage {
	if site == nil {
		site = NewSiteConfig("Blog")
	}

	// Mark Home as active
	site.Navigation[0].IsActive = true

	return &IndexPage{
		Site: *site,
		Meta: PageMeta{
			Title: title,
		},
		Posts: make([]PostSummary, 0),
	}
}

// NewPostsPage creates a new PostsPage with default values
func NewPostsPage(site *SiteConfig) *PostsPage {
	if site == nil {
		site = NewSiteConfig("Blog")
	}

	// Mark Posts as active
	site.Navigation[2].IsActive = true

	return &PostsPage{
		Site: *site,
		Meta: PageMeta{
			Title: "All Posts",
		},
		PostsByYear: make([]YearGroup, 0),
		AllTags:     make([]TagInfo, 0),
		Archives:    make([]Archive, 0),
	}
}

// SetActiveNavigation marks the specified URL as active and others as inactive
func (sc *SiteConfig) SetActiveNavigation(url string) {
	for i := range sc.Navigation {
		sc.Navigation[i].IsActive = (sc.Navigation[i].URL == url)
	}
}
