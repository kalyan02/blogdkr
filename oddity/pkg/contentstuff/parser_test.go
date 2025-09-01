package contentstuff

import (
	"strings"
	"testing"
)

func TestFrontmatterYAML(t *testing.T) {
	content := []byte(`---
title: Test Post
tags:
  - blog
  - markdown
published: true
---

# Hello World

This is a test post with **YAML** frontmatter.

#testing #yaml`)

	parser := NewMarkdownParser(DefaultParserConfig())
	result, err := parser.Parse(content)
	if err != nil {
		t.Fatalf("Failed to parse content: %v", err)
	}

	// Test frontmatter parsing
	if result.Frontmatter == nil {
		t.Fatal("Expected frontmatter to be parsed")
	}

	if result.Frontmatter.Type != FrontmatterYAML {
		t.Errorf("Expected YAML frontmatter, got %v", result.Frontmatter.Type)
	}

	// Test frontmatter data
	title, _ := result.Frontmatter.GetString("title")
	if title != "Test Post" {
		t.Errorf("Expected title 'Test Post', got '%s'", title)
	}

	tags := result.Frontmatter.GetStringSlice("tags")
	if len(tags) != 2 || tags[0] != "blog" || tags[1] != "markdown" {
		t.Errorf("Expected tags [blog, markdown], got %v", tags)
	}

	published := result.Frontmatter.GetBool("published")
	if !published {
		t.Errorf("Expected published to be true")
	}

	// Test body content (should not contain frontmatter)
	bodyStr := string(result.Body)
	if len(bodyStr) == 0 {
		t.Fatal("Expected body content")
	}

	if result.Title != "Hello World" {
		t.Errorf("Expected title to be 'Test Post', got '%s'", result.Title)
	}

	if !strings.Contains(bodyStr, "This is a test post with **YAML** frontmatter.") {
		t.Error("Body content does not match expected content")
	}

	// Test hashtags extraction
	if len(result.Hashtags) != 2 {
		t.Errorf("Expected 2 hashtags, got %d", len(result.Hashtags))
	}
	if result.Hashtags[0] != "testing" || result.Hashtags[1] != "yaml" {
		t.Errorf("Expected hashtags [testing, yaml], got %v", result.Hashtags)
	}
}

func TestFrontmatterTOML(t *testing.T) {
	content := []byte(`+++
title = "TOML Test Post"
tags = ["blog", "toml", "frontmatter"]
published = false
weight = 42
+++

# TOML Example

This post uses **TOML** frontmatter.

Check out this [[wiki link]] and #toml hashtag.`)

	parser := NewMarkdownParser(DefaultParserConfig())
	result, err := parser.Parse(content)
	if err != nil {
		t.Fatalf("Failed to parse content: %v", err)
	}

	// Test frontmatter parsing
	if result.Frontmatter == nil {
		t.Fatal("Expected frontmatter to be parsed")
	}

	if result.Frontmatter.Type != FrontmatterTOML {
		t.Errorf("Expected TOML frontmatter, got %v", result.Frontmatter.Type)
	}

	// Test frontmatter data
	title, _ := result.Frontmatter.GetString("title")
	if title != "TOML Test Post" {
		t.Errorf("Expected title 'TOML Test Post', got '%s'", title)
	}

	tags := result.Frontmatter.GetStringSlice("tags")
	if len(tags) != 3 || tags[0] != "blog" || tags[1] != "toml" || tags[2] != "frontmatter" {
		t.Errorf("Expected tags [blog, toml, frontmatter], got %v", tags)
	}

	published := result.Frontmatter.GetBool("published")
	if published {
		t.Errorf("Expected published to be false")
	}

	// Test HasKey method
	if !result.Frontmatter.HasKey("weight") {
		t.Error("Expected 'weight' key to exist")
	}
	if result.Frontmatter.HasKey("nonexistent") {
		t.Error("Expected 'nonexistent' key to not exist")
	}

	// Test hashtags
	if len(result.Hashtags) != 1 || result.Hashtags[0] != "toml" {
		t.Errorf("Expected hashtags [toml], got %v", result.Hashtags)
	}
}

func TestNoFrontmatter(t *testing.T) {
	content := []byte(`# Regular Post

This post has no frontmatter.

Just regular **markdown** content with #hashtags.`)

	parser := NewMarkdownParser(DefaultParserConfig())
	result, err := parser.Parse(content)
	if err != nil {
		t.Fatalf("Failed to parse content: %v", err)
	}

	// Should have no frontmatter
	if result.Frontmatter != nil {
		t.Error("Expected no frontmatter")
	}

	if result.Title != "Regular Post" {
		t.Errorf("Expected title 'Regular Post', got '%s'", result.Title)
	}

	bodyStr := string(result.Body)
	if !strings.Contains(bodyStr, "This post has no frontmatter.") {
		t.Error("Body content does not match expected content")
	}

	if strings.Contains(bodyStr, "Regular Post") {
		t.Error("Body should not contain the title")
	}

	// Should still extract hashtags
	if len(result.Hashtags) != 1 || result.Hashtags[0] != "hashtags" {
		t.Errorf("Expected hashtags [hashtags], got %v", result.Hashtags)
	}
}

func TestPlainTextExtraction(t *testing.T) {
	content := []byte(`---
title: Plain Text Test
---

# Main Title

This is a **bold** and *italic* text with [links](http://example.com).

- List item 1
- List item 2

> Quote block

Final paragraph.`)

	parser := NewMarkdownParser(DefaultParserConfig())
	result, err := parser.Parse(content)
	if err != nil {
		t.Fatalf("Failed to parse content: %v", err)
	}

	plainText := result.PlainText
	if len(plainText) == 0 {
		t.Fatal("Expected plain text content")
	}

	// Should contain text without markdown formatting
	expected := "This is a bold and italic text with links . List item 1 List item 2 Quote block Final paragraph."
	if plainText != expected {
		t.Errorf("Expected plain text '%s', got '%s'", expected, plainText)
	}
}

func TestConfigurableFeatures(t *testing.T) {
	content := []byte(`# Test

Content with [[wiki link]] and #hashtag.`)

	// Test with wiki links disabled
	config := DefaultParserConfig()
	config.EnableWikiLinks = false
	parser := NewMarkdownParser(config)
	result, err := parser.Parse(content)
	if err != nil {
		t.Fatalf("Failed to parse content: %v", err)
	}

	htmlStr := string(result.HTML)
	// Should contain literal [[wiki link]] instead of parsed link
	if !strings.Contains(htmlStr, "[[wiki link]]") {
		t.Logf("HTML output: %s", htmlStr)
		t.Error("Expected literal [[wiki link]] when wiki links disabled")
	}

	// Test with hashtags disabled
	config2 := DefaultParserConfig()
	config2.EnableHashtags = false
	parser2 := NewMarkdownParser(config2)
	result2, err := parser2.Parse(content)
	if err != nil {
		t.Fatalf("Failed to parse content: %v", err)
	}

	if len(result2.Hashtags) != 0 {
		t.Errorf("Expected no hashtags when disabled, got %v", result2.Hashtags)
	}

	t.Run("TestWikiLinkParsing", func(t *testing.T) {
		content := []byte(`# Wiki Link Test
This post has a [[SamplePage]] link.`)
		config := DefaultParserConfig()
		config.EnableWikiLinks = true
		parser := NewMarkdownParser(config)
		result, err := parser.Parse(content)
		if err != nil {
			t.Fatalf("Failed to parse content: %v", err)
		}
		htmlStr := string(result.HTML)
		if !strings.Contains(htmlStr, `<a href="SamplePage"`) {
			t.Errorf("Expected wiki link to be parsed, got HTML: %s", htmlStr)
		}
	})
}

func TestToMarkdown(t *testing.T) {
	content := []byte(`---
title: Markdown Roundtrip
tags:
  - test
  - markdown
---
# Heading
This is a **test** post with #hashtags and a [[WikiLink]].`)

	parser := NewMarkdownParser(DefaultParserConfig())
	result, err := parser.Parse(content)
	if err != nil {
		t.Fatalf("Failed to parse content: %v", err)
	}

	md, err := result.ToMarkdown()
	if err != nil {
		t.Fatalf("Failed to convert back to markdown: %v", err)
	}

	parser = NewMarkdownParser(DefaultParserConfig())
	result2, err := parser.Parse([]byte(md))
	if err != nil {
		t.Fatalf("Failed to parse roundtrip markdown: %v", err)
	}

	if result2.Title != result.Title {
		t.Errorf("Expected title '%s', got '%s' after roundtrip", result.Title, result2.Title)
	}
	if len(result2.Hashtags) != len(result.Hashtags) {
		t.Errorf("Expected %d hashtags, got %d after roundtrip", len(result.Hashtags), len(result2.Hashtags))
	}
	if string(result2.Body) != string(result.Body) {
		t.Errorf("Expected body to match after roundtrip.\nOriginal:\n%s\n\nGot:\n%s", result.Body, result2.Body)
	}

	// validate all frontmatter keys are the same
	for k, v := range result.Frontmatter.DataKV {
		v2, ok := result2.Frontmatter.DataKV[k]
		if !ok {
			t.Errorf("Expected frontmatter key '%s' to exist after roundtrip", k)
			continue
		}

		// if strings or ints, compare directly
		// for slices, compare length and elements
		// for other types, just use fmt.Sprintf to compare string representations
		if sv, ok := v.([]interface{}); ok {
			sv2, ok2 := v2.([]interface{})
			if !ok2 || len(sv) != len(sv2) {
				t.Errorf("Expected frontmatter key '%s' to have slice value '%v', got '%v' after roundtrip", k, v, v2)
				continue
			}
			for i := range sv {
				if sv[i] != sv2[i] {
					t.Errorf("Expected frontmatter key '%s' slice element %d to be '%s', got '%s' after roundtrip", k, i, sv[i], sv2[i])
				}
			}
			continue
		}

		if v != v2 {
			t.Errorf("Expected frontmatter key '%s' to have value '%v', got '%v' after roundtrip", k, v, v2)
		}
	}

}
