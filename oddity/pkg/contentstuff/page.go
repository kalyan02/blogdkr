package contentstuff

import (
	"html/template"
	"path"
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
	if firstL1 == "" {
		base := filepath.Base(p.File.FileName)
		firstL1 = strings.TrimSuffix(base, filepath.Ext(base))
	}

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
	body := []byte(`# ` + p.Title() + "\n")
	body = append(body, p.Body(false)...)
	return body
}

func (p *Page) BodyNoTitle() []byte {
	return p.Body(true)
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
		if intVal, ok := val.(int); ok && intVal != 0 {
			return timeFromMilliOrSeconds(int64(intVal)), true
		}
		if int64Val, ok := val.(int64); ok && int64Val != 0 {
			return timeFromMilliOrSeconds(int64Val), true
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
