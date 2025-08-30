package contentstuff

import (
	"html/template"
	"math"
	"path/filepath"
	"regexp"
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

	if p.File.ParsedContent != nil && len(p.File.ParsedContent.Title) > 0 {
		firstL1 = p.File.ParsedContent.Title
	}

	// check front matter title
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil {
		if title, ok := p.File.ParsedContent.Frontmatter.GetString("title"); ok && title != "" {
			firstL1 = title
		}
	}

	// use filename (without dir and extension) as title if no h1 or frontmatter title
	//if firstL1 == "" {
	//	base := filepath.Base(p.File.FileName)
	//	firstL1 = strings.TrimSuffix(base, filepath.Ext(base))
	//}

	return firstL1
}

var titleRegexp = regexp.MustCompile("(?m)^#\\s*(.*)\n+")

func (p *Page) Body(noTitle bool) []byte {
	s := string(p.File.ParsedContent.Body)
	if noTitle {
		m := titleRegexp.FindStringSubmatch(s)
		if m != nil {
			return []byte(strings.Replace(s, m[0], "", 1))
		}
	}
	return []byte(s)
}

func (p *Page) BodyWithTitle() []byte {

	body := []byte("")
	if title := p.Title(); title != "" {
		body = append(body, []byte(`# `+title+"\n")...)
	}

	body = append(body, p.Body(false)...)
	return body
}

func (p *Page) Hashtags() []string {
	if p.File.ParsedContent != nil && p.File.ParsedContent.Hashtags != nil {
		return p.File.ParsedContent.Hashtags
	}
	return nil
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
		if slug, ok := p.File.ParsedContent.Frontmatter.GetString("slug"); ok && slug != "" {
			pgslug = slug
			pgslug = filepath.Join(filepath.Dir(p.File.FileName), pgslug)
		}
	}
	if pgslug == "" {
		pgslug = strings.TrimSuffix(p.File.FileName, filepath.Ext(p.File.FileName))
	}
	return pgslug
}

func timeFromMilliOrSeconds(ts int64) time.Time {
	if ts > 1e12 {
		return time.Unix(ts/1000, 0)
	}
	return time.Unix(ts, 0)
}

func parseUnixMilliOrSeconds(ts string) (time.Time, error) {
	if len(ts) == 0 {
		return time.Time{}, nil
	}
	// try milliseconds
	if tsm, err := strconv.ParseInt(ts, 10, 64); err == nil {
		return timeFromMilliOrSeconds(tsm), nil
	} else {
		return time.Time{}, err
	}
}

func (p *Page) tryParseTimeField(field string) (time.Time, bool) {
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil && p.File.ParsedContent.Frontmatter.HasKey(field) {

		// if time.Time type

		if strVal, ok := p.File.ParsedContent.Frontmatter.GetString(field); ok && strVal != "" {
			if t, err := parseUnixMilliOrSeconds(strVal); err == nil && !t.IsZero() {
				return t, true
			}
		}
		// if integer type
		val, _ := p.File.ParsedContent.Frontmatter.GetValue(field)
		switch v := val.(type) {
		case int:
			if v != 0 {
				return timeFromMilliOrSeconds(int64(v)), true
			}
		case int32:
			if v != 0 {
				return timeFromMilliOrSeconds(int64(v)), true
			}
		case int64:
			if v != 0 {
				return timeFromMilliOrSeconds(v), true
			}
		case uint:
			if v != 0 {
				return timeFromMilliOrSeconds(int64(v)), true
			}
		case uint32:
			if v != 0 {
				return timeFromMilliOrSeconds(int64(v)), true
			}
		case uint64:
			if v != 0 && v <= math.MaxInt64 { // prevent overflow
				return timeFromMilliOrSeconds(int64(v)), true
			}
		}
	}
	return time.Time{}, false
}

func (p *Page) DateCreated() *time.Time {
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil {
		if created, ok := p.tryParseTimeField("created"); ok {
			return &created
		}
		if created, ok := p.tryParseTimeField("_created"); ok {
			return &created
		}
		if date, ok := p.tryParseTimeField("date"); ok {
			return &date
		}
	}
	if !p.File.CreatedAt.IsZero() {
		return &p.File.CreatedAt
	}

	return nil
}

func (p *Page) DateModified() *time.Time {
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil {
		if modified, ok := p.tryParseTimeField("modified"); ok {
			return &modified
		}
		if modified, ok := p.tryParseTimeField("_modified"); ok {
			return &modified
		}
		if updated, ok := p.tryParseTimeField("updated"); ok {
			return &updated
		}
	}
	if !p.File.ModifiedAt.IsZero() {
		return &p.File.ModifiedAt
	}
	return nil
}

// IsPrivate checks if the page is marked as private in frontmatter
func (p *Page) IsPrivate() bool {
	if p.File.ParsedContent != nil && p.File.ParsedContent.Frontmatter != nil {
		return p.File.ParsedContent.Frontmatter.GetBool("private")
	}
	return false
}
