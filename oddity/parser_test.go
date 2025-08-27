package main

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
	title := result.Frontmatter.GetString("title")
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
	if bodyStr[0:2] != "# " {
		t.Errorf("Expected body to start with '# ', got '%s'", bodyStr[0:10])
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
	title := result.Frontmatter.GetString("title")
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

	// Body should be the same as original content
	if string(result.Body) != string(content) {
		t.Error("Body should match original content when no frontmatter")
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
	expected := "Main Title This is a bold and italic text with links . List item 1 List item 2 Quote block Final paragraph."
	if plainText != expected {
		t.Errorf("Expected plain text '%s', got '%s'", expected, plainText)
	}
}

func TestLegacyCompatibility(t *testing.T) {
	// Test that the legacy Page methods still work
	page := &Page{
		Body: []byte(`---
title: Legacy Test
---

# Legacy Page

Content with #legacy hashtag.`),
	}

	// Test renderHtml
	page.renderHtml()
	if len(page.HTML) == 0 {
		t.Error("Expected HTML content from renderHtml")
	}
	if len(page.Hashtags) != 1 || page.Hashtags[0] != "legacy" {
		t.Errorf("Expected hashtags [legacy], got %v", page.Hashtags)
	}

	// Test plainText
	plainText := page.plainText()
	if len(plainText) == 0 {
		t.Error("Expected plain text from plainText method")
	}

	// Test images (should return empty for this content)
	images := page.images()
	if len(images) != 0 {
		t.Errorf("Expected no images, got %d", len(images))
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
