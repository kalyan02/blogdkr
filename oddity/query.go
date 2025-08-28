package main

import (
	"encoding/xml"
	"fmt"
	"strconv"
	"strings"
)

// QueryType represents the type of query
type QueryType int

const (
	QueryPosts QueryType = iota
	QueryPages
	QueryBacklinks
)

func (qt QueryType) String() string {
	switch qt {
	case QueryPosts:
		return "posts"
	case QueryPages:
		return "pages"
	case QueryBacklinks:
		return "backlinks"
	default:
		return "unknown"
	}
}

// SortType represents sorting options
type SortType string

const (
	SortRecent   SortType = "recent"
	SortDate     SortType = "date"
	SortModified SortType = "modified"
	SortTitle    SortType = "title"
)

// SortOrder represents sort direction
type SortOrder string

const (
	SortAsc  SortOrder = "asc"
	SortDesc SortOrder = "desc"
)

// FormatType represents markdown output format
type FormatType string

const (
	FormatList         FormatType = "list"
	FormatTable        FormatType = "table"
	FormatListWithDate FormatType = "list-date"
	FormatDetailed     FormatType = "detailed"
)

// QueryFilter represents a filter condition
type QueryFilter struct {
	Field    string `json:"field"`
	Operator string `json:"operator"` // contains, equals, etc.
	Value    string `json:"value"`
}

// QueryAST represents a parsed query with all its parameters
type QueryAST struct {
	Type         QueryType     `json:"type"`
	Path         string        `json:"path,omitempty"` // path filter pattern
	Sort         SortType      `json:"sort,omitempty"`
	Order        SortOrder     `json:"order,omitempty"`
	Limit        int           `json:"limit,omitempty"`
	Filters      []QueryFilter `json:"filters,omitempty"`
	HTMLTemplate string        `json:"html_template,omitempty"`
	MDFormat     FormatType    `json:"md_format,omitempty"`
}

// QueryXML represents the XML structure for parsing
type QueryXML struct {
	XMLName      xml.Name `xml:"query"`
	Type         string   `xml:"type,attr"`
	Path         string   `xml:"path,attr"`
	Sort         string   `xml:"sort,attr"`
	Order        string   `xml:"order,attr"`
	Limit        string   `xml:"limit,attr"`
	HTMLTemplate string   `xml:"html-template,attr"`
	MDFormat     string   `xml:"md-format,attr"`
	Where        string   `xml:"where,attr"`
	Tag          string   `xml:"tag,attr"`
}

// ParseQuery parses a query string in XML format
func ParseQuery(queryString string) (*QueryAST, error) {
	// Clean up the query string - remove HTML comment markers
	queryString = strings.TrimSpace(queryString)
	queryString = strings.TrimPrefix(queryString, "<!--")
	queryString = strings.TrimSuffix(queryString, "-->")
	queryString = strings.TrimSpace(queryString)

	// Convert non-self-closing tags to self-closing for XML parsing
	if !strings.HasSuffix(queryString, "/>") && strings.HasSuffix(queryString, ">") {
		queryString = strings.TrimSuffix(queryString, ">") + "/>"
	}

	// Parse as XML
	var queryXML QueryXML
	if err := xml.Unmarshal([]byte(queryString), &queryXML); err != nil {
		return nil, fmt.Errorf("failed to parse query XML: %v", err)
	}

	// Convert to QueryAST
	query := &QueryAST{
		HTMLTemplate: queryXML.HTMLTemplate,
		Path:         queryXML.Path,
	}

	// Parse query type
	switch strings.ToLower(queryXML.Type) {
	case "posts":
		query.Type = QueryPosts
	case "pages":
		query.Type = QueryPages
	case "backlinks":
		query.Type = QueryBacklinks
	default:
		return nil, fmt.Errorf("unknown query type: %s", queryXML.Type)
	}

	// Parse sort type
	if queryXML.Sort != "" {
		query.Sort = SortType(strings.ToLower(queryXML.Sort))
	} else {
		query.Sort = SortRecent // default
	}

	// Parse sort order
	if queryXML.Order != "" {
		query.Order = SortOrder(strings.ToLower(queryXML.Order))
	} else {
		// Default order based on sort type
		if query.Sort == SortRecent || query.Sort == SortDate || query.Sort == SortModified {
			query.Order = SortDesc
		} else {
			query.Order = SortAsc
		}
	}

	// Parse limit
	if queryXML.Limit != "" {
		if limit, err := strconv.Atoi(queryXML.Limit); err == nil {
			query.Limit = limit
		}
	}

	// Parse markdown format
	if queryXML.MDFormat != "" {
		query.MDFormat = FormatType(strings.ToLower(queryXML.MDFormat))
	} else {
		query.MDFormat = FormatListWithDate // default
	}

	// Parse filters
	if queryXML.Where != "" {
		filter, err := parseWhereClause(queryXML.Where)
		if err != nil {
			return nil, fmt.Errorf("failed to parse where clause: %v", err)
		}
		query.Filters = append(query.Filters, *filter)
	}

	// Handle tag filter (shorthand for where tag contains)
	if queryXML.Tag != "" {
		filter := QueryFilter{
			Field:    "tag",
			Operator: "contains",
			Value:    queryXML.Tag,
		}
		query.Filters = append(query.Filters, filter)
	}

	return query, nil
}

// parseWhereClause parses a simple where clause like "tag contains 'project'"
func parseWhereClause(whereClause string) (*QueryFilter, error) {
	parts := strings.Fields(whereClause)
	if len(parts) < 3 {
		return nil, fmt.Errorf("invalid where clause: %s", whereClause)
	}

	field := parts[0]
	operator := parts[1]
	value := strings.Join(parts[2:], " ")

	// Remove quotes from value
	value = strings.Trim(value, `"'`)

	return &QueryFilter{
		Field:    field,
		Operator: operator,
		Value:    value,
	}, nil
}

// String returns a string representation of the query
func (q *QueryAST) String() string {
	parts := []string{q.Type.String()}

	if q.Path != "" {
		parts = append(parts, fmt.Sprintf("path:%s", q.Path))
	}

	if q.Sort != "" {
		parts = append(parts, fmt.Sprintf("sort:%s", q.Sort))
		if q.Order != "" && q.Order != SortAsc {
			parts = append(parts, fmt.Sprintf("order:%s", q.Order))
		}
	}

	if q.Limit > 0 {
		parts = append(parts, fmt.Sprintf("limit:%d", q.Limit))
	}

	for _, filter := range q.Filters {
		parts = append(parts, fmt.Sprintf("where:%s %s '%s'", filter.Field, filter.Operator, filter.Value))
	}

	if q.HTMLTemplate != "" {
		parts = append(parts, fmt.Sprintf("template:%s", q.HTMLTemplate))
	}

	if q.MDFormat != "" && q.MDFormat != FormatList {
		parts = append(parts, fmt.Sprintf("format:%s", q.MDFormat))
	}

	return strings.Join(parts, " ")
}
