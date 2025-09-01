package contentstuff

import (
	"fmt"
	"regexp"

	"github.com/BurntSushi/toml"
	"github.com/goccy/go-yaml"
)

// FrontmatterType represents the type of frontmatter delimiter used
type FrontmatterType int

const (
	FrontmatterNone FrontmatterType = iota
	FrontmatterYAML                 // --- delimited YAML
	FrontmatterTOML                 // +++ delimited TOML
)

// FrontmatterData holds parsed frontmatter content and metadata
type FrontmatterData struct {
	Type     FrontmatterType
	Raw      string
	Data     yaml.MapSlice
	DataKV   map[string]interface{}
	StartPos int
	EndPos   int
}

func (fm *FrontmatterData) SetRaw(data []byte) error {
	fm.Raw = string(data)
	switch fm.Type {
	case FrontmatterYAML:
		return yaml.Unmarshal(data, &fm.Data)
	case FrontmatterTOML:
		return toml.Unmarshal(data, &fm.Data)
	default:
		return nil
	}
}

// GetString safely gets a string value from frontmatter data
func (fm *FrontmatterData) GetString(key string) (string, bool) {
	if fm == nil || fm.Data == nil {
		return "", false
	}
	if val, ok := fm.DataKV[key]; ok {
		if str, ok := val.(string); ok {
			return str, true
		}
	}
	return "", false
}

// SetString safely sets a string value in frontmatter data
func (fm *FrontmatterData) SetString(key, value string) {
	fm.SetValue(key, value)
}

// SetValue safely sets a value of any type in frontmatter data
func (fm *FrontmatterData) SetValue(key string, value any) {
	if fm == nil {
		return
	}
	if fm.DataKV == nil {
		fm.DataKV = make(map[string]interface{})
	}

	// if fm.DataKV[key] doesn't exist, add it only the first time as it helps preserve order
	if _, exists := fm.DataKV[key]; !exists {
		fm.Data = append(fm.Data, yaml.MapItem{Key: key, Value: value})
	}

	fm.DataKV[key] = value
}

// GetValue safely gets a value of any type from frontmatter data
func (fm *FrontmatterData) GetValue(key string) (any, bool) {
	if fm == nil || fm.Data == nil {
		return nil, false
	}
	val, ok := fm.DataKV[key]
	return val, ok
}

// GetStringSlice safely gets a string slice from frontmatter data
func (fm *FrontmatterData) GetStringSlice(key string) []string {
	if fm == nil || fm.Data == nil {
		return nil
	}
	if val, ok := fm.DataKV[key]; ok {
		switch v := val.(type) {
		case []string:
			return v
		case []interface{}:
			result := make([]string, 0, len(v))
			for _, item := range v {
				if str, ok := item.(string); ok {
					result = append(result, str)
				}
			}
			return result
		}
	}
	return nil
}

// GetBool safely gets a boolean value from frontmatter data
func (fm *FrontmatterData) GetBool(key string) bool {
	if fm == nil || fm.Data == nil {
		return false
	}
	if val, ok := fm.DataKV[key]; ok {
		if b, ok := val.(bool); ok {
			return b
		}
	}
	return false
}

// HasKey checks if a key exists in frontmatter data
func (fm *FrontmatterData) HasKey(key string) bool {
	if fm == nil || fm.Data == nil {
		return false
	}
	_, exists := fm.DataKV[key]
	return exists
}

// Marshal the frontmatter data to string
func (fm *FrontmatterData) Marshal() (string, error) {
	if fm == nil {
		return "", nil
	}

	// sync DataKV to Data
	// first update existing keys
	var updatedKeys = make(map[string]bool)
	for i, item := range fm.Data {
		// TODO: maybe DataKV doesn't have to have string keys
		keyString, ok := item.Key.(string)
		if !ok {
			continue
		}

		if val, ok := fm.DataKV[keyString]; ok {
			fm.Data[i].Value = val
			updatedKeys[keyString] = true
		} else {
			// remove key from Data if not in DataKV
			fm.Data = append(fm.Data[:i], fm.Data[i+1:]...)
		}
	}
	// then add new keys
	for key, val := range fm.DataKV {
		if _, ok := updatedKeys[key]; !ok {
			fm.Data = append(fm.Data, yaml.MapItem{Key: key, Value: val})
		}
	}

	out, err := yaml.Marshal(fm.Data)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("---\n%s---", string(out)), nil

}

// ExtractFrontmatter extracts and parses frontmatter from content
func ExtractFrontmatter(content []byte) (*FrontmatterData, []byte, error) {
	if len(content) == 0 {
		return nil, content, nil
	}

	// Check for YAML frontmatter (---)
	// Use (?s) flag for . to match newlines
	if yamlMatch := regexp.MustCompile(`(?s)^---\s*\n(.*?)\n---\s*\n`).FindSubmatch(content); yamlMatch != nil {
		frontmatter := &FrontmatterData{
			Type:   FrontmatterYAML,
			EndPos: len(yamlMatch[0]),
			Data:   make(yaml.MapSlice, 0),
			DataKV: make(map[string]interface{}),
		}

		if err := frontmatter.SetRaw(yamlMatch[1]); err != nil {
			return nil, content, fmt.Errorf("failed to parse YAML frontmatter: %w", err)
		}

		body := content[frontmatter.EndPos:]
		return frontmatter, body, nil
	}

	// Check for TOML frontmatter (+++)
	// Use (?s) flag for . to match newlines
	if tomlMatch := regexp.MustCompile(`(?s)^\+\+\+\s*\n(.*?)\n\+\+\+\s*\n`).FindSubmatch(content); tomlMatch != nil {
		frontmatter := &FrontmatterData{
			Type:   FrontmatterTOML,
			EndPos: len(tomlMatch[0]),
			Data:   make(yaml.MapSlice, 0),
			DataKV: make(map[string]interface{}),
		}

		if err := frontmatter.SetRaw(tomlMatch[1]); err != nil {
			return nil, content, fmt.Errorf("failed to parse TOML frontmatter: %w", err)
		}

		body := content[frontmatter.EndPos:]
		return frontmatter, body, nil
	}

	// No frontmatter found
	return nil, content, nil
}
