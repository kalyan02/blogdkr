package main

import (
	"testing"
)

func TestParseQuery(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		expectErr bool
		expected  *QueryAST
	}{
		{
			name:      "Simple posts query",
			input:     `<query type="posts" limit="5">`,
			expectErr: false,
			expected: &QueryAST{
				Type:     QueryPosts,
				Limit:    5,
				MDFormat: FormatListWithDate,
				Sort:     SortRecent,
				Order:    SortDesc,
			},
		},
		{
			name:      "Posts query with sorting",
			input:     `<query type="posts" sort="recent" limit="3" order="desc">`,
			expectErr: false,
			expected: &QueryAST{
				Type:     QueryPosts,
				Sort:     SortRecent,
				Order:    SortDesc,
				Limit:    3,
				MDFormat: FormatListWithDate,
			},
		},
		{
			name:      "Posts query with template and format",
			input:     `<query type="posts" html-template="recent-posts.html" md-format="list" limit="10">`,
			expectErr: false,
			expected: &QueryAST{
				Type:         QueryPosts,
				HTMLTemplate: "recent-posts.html",
				MDFormat:     FormatList,
				Limit:        10,
				Order:        SortDesc,
				Sort:         SortRecent,
			},
		},
		{
			name:      "Backlinks query",
			input:     `<query type="backlinks">`,
			expectErr: false,
			expected: &QueryAST{
				Type:     QueryBacklinks,
				MDFormat: FormatListWithDate,
				Order:    SortDesc,
				Sort:     SortRecent,
			},
		},
		{
			name:      "Posts query with tag filter",
			input:     `<query type="posts" tag="project" sort="date" order="desc">`,
			expectErr: false,
			expected: &QueryAST{
				Type:     QueryPosts,
				Sort:     SortDate,
				Order:    SortDesc,
				MDFormat: FormatListWithDate,
				Filters: []QueryFilter{
					{Field: "tag", Operator: "contains", Value: "project"},
				},
			},
		},
		{
			name:      "Posts query with where clause",
			input:     `<query type="posts" where="tag contains 'golang'" limit="5">`,
			expectErr: false,
			expected: &QueryAST{
				Type:     QueryPosts,
				Limit:    5,
				MDFormat: FormatListWithDate,
				Order:    SortDesc,
				Filters: []QueryFilter{
					{Field: "tag", Operator: "contains", Value: "golang"},
				},
				Sort: SortRecent,
			},
		},
		{
			name:      "Posts query with path filter",
			input:     `<query type="posts" path="blog/*" sort="recent" limit="3">`,
			expectErr: false,
			expected: &QueryAST{
				Type:     QueryPosts,
				Path:     "blog/*",
				Sort:     SortRecent,
				Order:    SortDesc,
				Limit:    3,
				MDFormat: FormatListWithDate,
			},
		},
		{
			name:      "Posts query with complex path pattern",
			input:     `<query type="posts" path="./notes/*.md" md-format="detailed">`,
			expectErr: false,
			expected: &QueryAST{
				Type:     QueryPosts,
				Path:     "./notes/*.md",
				MDFormat: FormatDetailed,
				Order:    SortDesc,
				Sort:     SortRecent,
			},
		},
		{
			name:      "Invalid query type",
			input:     `<query type="invalid">`,
			expectErr: true,
		},
		{
			name:      "Non-self-closing tag",
			input:     `<query type="posts" limit="5">`,
			expectErr: false,
			expected: &QueryAST{
				Type:     QueryPosts,
				Limit:    5,
				MDFormat: FormatListWithDate,
				Order:    SortDesc,
				Sort:     SortRecent,
			},
		},
		{
			name:      "Malformed XML",
			input:     `<query type="posts" limit="5"`,
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := ParseQuery(tt.input)

			if tt.expectErr {
				if err == nil {
					t.Errorf("Expected error but got none")
				}
				return
			}

			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}

			// Compare key fields
			if result.Type != tt.expected.Type {
				t.Errorf("Type mismatch: got %v, want %v", result.Type, tt.expected.Type)
			}

			if result.Path != tt.expected.Path {
				t.Errorf("Path mismatch: got %v, want %v", result.Path, tt.expected.Path)
			}

			if result.Sort != tt.expected.Sort {
				t.Errorf("Sort mismatch: got %v, want %v", result.Sort, tt.expected.Sort)
			}

			if result.Order != tt.expected.Order {
				t.Errorf("Order mismatch: got %v, want %v", result.Order, tt.expected.Order)
			}

			if result.Limit != tt.expected.Limit {
				t.Errorf("Limit mismatch: got %v, want %v", result.Limit, tt.expected.Limit)
			}

			if result.HTMLTemplate != tt.expected.HTMLTemplate {
				t.Errorf("HTMLTemplate mismatch: got %v, want %v", result.HTMLTemplate, tt.expected.HTMLTemplate)
			}

			if result.MDFormat != tt.expected.MDFormat {
				t.Errorf("MDFormat mismatch: got %v, want %v", result.MDFormat, tt.expected.MDFormat)
			}

			// Check filters length
			if len(result.Filters) != len(tt.expected.Filters) {
				t.Errorf("Filters length mismatch: got %d, want %d", len(result.Filters), len(tt.expected.Filters))
			}

			// Check individual filters
			for i, filter := range result.Filters {
				if i >= len(tt.expected.Filters) {
					break
				}
				expected := tt.expected.Filters[i]
				if filter.Field != expected.Field || filter.Operator != expected.Operator || filter.Value != expected.Value {
					t.Errorf("Filter %d mismatch: got {%s %s %s}, want {%s %s %s}",
						i, filter.Field, filter.Operator, filter.Value,
						expected.Field, expected.Operator, expected.Value)
				}
			}
		})
	}
}

func TestQueryString(t *testing.T) {
	query := &QueryAST{
		Type:     QueryPosts,
		Sort:     SortRecent,
		Order:    SortDesc,
		Limit:    5,
		MDFormat: FormatListWithDate,
		Filters: []QueryFilter{
			{Field: "tag", Operator: "contains", Value: "golang"},
		},
		HTMLTemplate: "recent-posts.html",
	}

	result := query.String()
	expected := "posts sort:recent order:desc limit:5 where:tag contains 'golang' template:recent-posts.html format:list-date"

	if result != expected {
		t.Errorf("String representation mismatch:\ngot:  %s\nwant: %s", result, expected)
	}
}

// Example usage documentation
func ExampleParseQuery() {
	// Basic posts query
	query1, _ := ParseQuery(`<query type="posts" limit="5">`)
	println("Query 1:", query1.String())

	// Posts with sorting and custom template
	query2, _ := ParseQuery(`<query type="posts" sort="recent" limit="3" html-template="card-layout.html" md-format="detailed">`)
	println("Query 2:", query2.String())

	// Posts with tag filter
	query3, _ := ParseQuery(`<query type="posts" tag="project" sort="date" order="desc">`)
	println("Query 3:", query3.String())

	// Backlinks query
	query4, _ := ParseQuery(`<query type="backlinks">`)
	println("Query 4:", query4.String())

	// Posts with path filtering
	query5, _ := ParseQuery(`<query type="posts" path="blog/*" sort="recent" limit="5">`)
	println("Query 5:", query5.String())
}

// TestQueryExtraction tests the complete extraction flow from markdown content
func TestQueryExtraction(t *testing.T) {
	tests := []struct {
		name            string
		markdownContent string
		expectedQueries int
		expectedQuery   *QueryAST
		expectedContent []string
	}{
		{
			name: "Complete query block",
			markdownContent: `# Test Page

<!-- <query type="posts" limit="3"> -->
- [Post 1](post1)
- [Post 2](post2)
<!-- </query> -->

Some other content.`,
			expectedQueries: 1,
			expectedQuery: &QueryAST{
				Type:     QueryPosts,
				Limit:    3,
				MDFormat: FormatList,
				Order:    SortAsc,
			},
			expectedContent: []string{
				"- [Post 1](post1)",
				"- [Post 2](post2)",
			},
		},
		{
			name: "Query with path filtering",
			markdownContent: `# Index Page

<!-- <query type="posts" path="blog/*" sort="recent" limit="2"> -->
- [Blog Post 1](blog/post1) - 2024-01-15
- [Blog Post 2](blog/post2) - 2024-01-14
<!-- </query> -->

End of page.`,
			expectedQueries: 1,
			expectedQuery: &QueryAST{
				Type:     QueryPosts,
				Path:     "blog/*",
				Sort:     SortRecent,
				Order:    SortDesc,
				Limit:    2,
				MDFormat: FormatList,
			},
			expectedContent: []string{
				"- [Blog Post 1](blog/post1) - 2024-01-15",
				"- [Blog Post 2](blog/post2) - 2024-01-14",
			},
		},
		{
			name: "Invalid query - missing end tag",
			markdownContent: `# Test Page

<!-- <query type="posts" limit="3"> -->
- [Post 1](post1)
- [Post 2](post2)

Some other content.`,
			expectedQueries: 0, // Should be skipped due to missing end tag
		},
		{
			name: "Invalid query syntax",
			markdownContent: `# Test Page

<!-- <query type="invalid-type" limit="abc"> -->
- [Post 1](post1)
<!-- </query> -->`,
			expectedQueries: 0, // Should be skipped due to invalid syntax
		},
		{
			name: "Multiple queries",
			markdownContent: `# Test Page

<!-- <query type="posts" limit="2"> -->
- [Post 1](post1)
- [Post 2](post2)
<!-- </query> -->

## Backlinks

<!-- <query type="backlinks"> -->
<!-- </query> -->

End of page.`,
			expectedQueries: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a mock Wire instance
			contentStuff := &ContentStuff{
				FileName: make(map[string]FileDetail),
				Config:   Config{ContentDir: "/test"},
			}
			wire := NewWire(contentStuff)

			// Create a mock FileDetail
			//fileDetail := FileDetail{
			//	FileName: "test.md",
			//	FileType: FileTypeMarkdown,
			//}

			// Test extraction using content directly
			queries, err := wire.extractQueriesFromContent("test.md", tt.markdownContent)
			if err != nil {
				t.Fatalf("Unexpected error during extraction: %v", err)
			}

			// Verify number of queries
			if len(queries) != tt.expectedQueries {
				t.Errorf("Expected %d queries, got %d", tt.expectedQueries, len(queries))
			}

			// If we expect queries, test the first one
			if tt.expectedQueries > 0 && len(queries) > 0 {
				query := queries[0]

				if tt.expectedQuery != nil {
					if query.Query.Type != tt.expectedQuery.Type {
						t.Errorf("Query type mismatch: got %v, want %v", query.Query.Type, tt.expectedQuery.Type)
					}
					if query.Query.Limit != tt.expectedQuery.Limit {
						t.Errorf("Query limit mismatch: got %v, want %v", query.Query.Limit, tt.expectedQuery.Limit)
					}
					if query.Query.Path != tt.expectedQuery.Path {
						t.Errorf("Query path mismatch: got %v, want %v", query.Query.Path, tt.expectedQuery.Path)
					}
				}

				// Check content if specified
				if tt.expectedContent != nil {
					if len(query.Content) != len(tt.expectedContent) {
						t.Errorf("Content length mismatch: got %d, want %d", len(query.Content), len(tt.expectedContent))
					}
					for i, line := range tt.expectedContent {
						if i < len(query.Content) && query.Content[i] != line {
							t.Errorf("Content line %d mismatch: got %q, want %q", i, query.Content[i], line)
						}
					}
				}
			}
		})
	}
}
