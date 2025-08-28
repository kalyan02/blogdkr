package main

import (
	"html/template"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type ImageData struct {
	Title, Name string
	Html        template.HTML
}

type Page struct {
	File *FileDetail
}

func NewPageFromFileDetail(f *FileDetail) *Page {
	return &Page{File: f}
}

func (p *Page) Title() string {
	firstL1 := ""

	mdParser := NewMarkdownParser(DefaultParserConfig())
	headings := mdParser.ExtractHeadings(p.File.ParsedContent.Body)
	if len(headings) > 0 {
		for _, h := range headings {
			if h.Level == 1 {
				firstL1 = h.Text
				break
			}
		}
	}

	// check front matter title
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil {
		if title, ok := p.File.ParsedContent.Frontmatter.Data["title"].(string); ok && title != "" {
			firstL1 = title
		}
	}

	// use filename (wihtout dir and extension) as title if no h1 or frontmatter title
	if firstL1 == "" {
		base := filepath.Base(p.File.FileName)
		firstL1 = strings.TrimSuffix(base, filepath.Ext(base))
	}

	return firstL1
}

func (p *Page) Hashtags() []string {
	if p.File.ParsedContent != nil && p.File.ParsedContent.Hashtags != nil {
		return p.File.ParsedContent.Hashtags
	}
	return nil
}

func (p *Page) DateCreated() time.Time {
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil {
		if created, ok := p.File.ParsedContent.Frontmatter.Data["created"].(string); ok && created != "" {
			if ts, err := strconv.ParseInt(created, 10, 64); err == nil {
				return time.Unix(ts/1000, 0)
			}
		}
	}
	if !p.File.CreatedAt.IsZero() {
		return p.File.CreatedAt
	}

	return p.File.ModifiedAt
}

func (p *Page) SafeHTML() template.HTML {
	if p.File.ParsedContent != nil {
		return template.HTML(p.File.ParsedContent.HTML)
	}
	return ""
}

func (p *Page) Slug() string {
	pgslug := ""
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil {
		if slug, ok := p.File.ParsedContent.Frontmatter.Data["slug"].(string); ok && slug != "" {
			pgslug = slug
			pgslug = filepath.Join(filepath.Dir(p.File.FileName), pgslug)
		}
	}
	if pgslug == "" {
		pgslug = strings.TrimSuffix(filepath.Base(p.File.FileName), filepath.Ext(p.File.FileName))
	}
	return pgslug
}

func parseUnixMilliOrSeconds(ts string) (time.Time, error) {
	if len(ts) == 0 {
		return time.Time{}, nil
	}
	// try milliseconds
	if tsm, err := strconv.ParseInt(ts, 10, 64); err == nil {
		if tsm > 1e12 {
			return time.Unix(tsm/1000, 0), nil
		}
		return time.Unix(tsm, 0), nil
	} else {
		return time.Time{}, err
	}
}

func (p *Page) DateModified() *time.Time {
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil {
		if modified, ok := p.File.ParsedContent.Frontmatter.Data["modified"].(string); ok && modified != "" {
			if t, err := parseUnixMilliOrSeconds(modified); err == nil && !t.IsZero() {
				return &t
			}
		}
		// updated
		if updated, ok := p.File.ParsedContent.Frontmatter.Data["updated"].(string); ok && updated != "" {
			if t, err := parseUnixMilliOrSeconds(updated); err == nil && !t.IsZero() {
				return &t
			}
		}
	}
	if !p.File.ModifiedAt.IsZero() {
		return &p.File.ModifiedAt
	}
	return nil
}

func (p *Page) Dir() string {
	d := path.Dir(p.File.FileName)
	if d == "." {
		return ""
	}
	return pathEncode(d) + "/"
}

const upperhex = "0123456789ABCDEF"

// pathEncode returns the page name with some characters escaped because html/template doesn't escape those. This is
// suitable for use in HTML templates.
func pathEncode(s string) string {
	n := strings.Count(s, ";") + strings.Count(s, ",") + strings.Count(s, "?") + strings.Count(s, "#")
	if n == 0 {
		return s
	}
	t := make([]byte, len(s)+2*n)
	j := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case ';', ',', '?', '#':
			t[j] = '%'
			t[j+1] = upperhex[s[i]>>4]
			t[j+2] = upperhex[s[i]&15]
			j += 3
		default:
			t[j] = s[i]
			j++
		}
	}
	return string(t)
}
