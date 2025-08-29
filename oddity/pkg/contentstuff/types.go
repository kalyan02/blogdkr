package contentstuff

import (
	"html/template"
	"time"

	"oddity/pkg/config"
)

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
	Site config.SiteConfig `json:"site"`
	Meta PageMeta          `json:"meta"`

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
	Site config.SiteConfig `json:"site"`
	Meta PageMeta          `json:"meta"`

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
	Site config.SiteConfig `json:"site"`
	Meta PageMeta          `json:"meta"`

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
