package main

import (
	"html/template"
	"path"
	"strings"
)

type ImageData struct {
	Title, Name string
	Html        template.HTML
}

type Page struct {
	Title    string
	FileName string //from content root
	Body     []byte
	HTML     template.HTML
	Hashtags []string
}

func (p *Page) Dir() string {
	d := path.Dir(p.FileName)
	if d == "." {
		return ""
	}
	return pathEncode(d) + "/"
}

// plainText renders the Page.Body to plain text
func (p *Page) plainText() string {
	parser := NewMarkdownParser(DefaultParserConfig())
	content, err := parser.Parse(p.Body)
	if err != nil {
		// Fallback to basic plain text conversion
		return string(p.Body)
	}
	return content.PlainText
}

// images returns an array of ImageData found in the page
func (p *Page) images() []ImageData {
	parser := NewMarkdownParser(DefaultParserConfig())
	content, err := parser.Parse(p.Body)
	if err != nil {
		return []ImageData{}
	}
	return content.Images
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
