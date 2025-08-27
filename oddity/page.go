package main

import (
	"html/template"
	"path"
	"strings"
)

type Frontmatter struct {
	Data map[string]string `yaml:",inline"`
}

type Link struct {
	Name   string
	URL    string
	Exists bool
	Class  string
}

type ImageData struct {
	Title, Name string
	Html        template.HTML
}

type Page struct {
	Title       string
	FileName    string //from content root
	Body        []byte
	HTML        template.HTML
	Hashtags    []string
	Frontmatter Frontmatter
}

func (p *Page) Dir() string {
	d := path.Dir(p.FileName)
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
