package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	toml "github.com/pelletier/go-toml/v2"

	"oddity/pkg/utils"
)

type Config struct {
	Content ContentConfig `toml:"content"`
	Site    SiteConfig    `toml:"site"`

	filePath string
}

func (c Config) EncodeTOML() ([]byte, error) {
	var buf bytes.Buffer
	encoder := toml.NewEncoder(&buf)
	encoder.SetIndentTables(false)
	if err := encoder.Encode(c); err != nil {
		return nil, fmt.Errorf("failed to marshal config to TOML: %w", err)
	}
	return buf.Bytes(), nil
}

func NewDefaultConfig() Config {
	return Config{
		Content: DefaultConfig,
		Site:    DefaultSiteConfig,
	}
}

func LoadConfigTOML(path string) (Config, error) {
	// if path is not absolute, get absolute path
	if !filepath.IsAbs(path) {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return Config{}, fmt.Errorf("failed to get absolute path of config file: %w", err)
		}
		path = absPath
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config file: %w", err)
	}

	cfg.filePath = path

	return cfg, nil
}

// ResolveDir resolves a directory path relative to the config file location if it's not absolute.
func (c Config) ResolveDir(path string) (string, error) {
	confPath := c.filePath

	return utils.ResolveRelative(path, confPath)
}

type ContentConfig struct {
	ContentDir     string   `toml:"content_dir"`
	StaticDirs     []string `toml:"static_dirs"`
	UploadDir      string   `toml:"upload_dir,omitempty"`
	ThemeDir       string   `toml:"theme_dir,omitempty"`
	Addr           string   `toml:"addr"`
	DefaultNewHint string   `toml:"default_new_hint"`
	SidecarDB      string   `toml:"sidecar_db"`
	AdminAddr      string   `toml:"admin_addr,omitempty"`
}

type SiteConfig struct {
	Title       string           `toml:"title"`
	Description string           `toml:"description,omitempty"`
	BaseURL     string           `toml:"base_url,omitempty"`
	Navigation  []NavigationLink `toml:"navigation,inline"`
	AuthorEmail string           `toml:"author_email,omitempty"`
	Author      string           `toml:"author"`
}

type NavigationLink struct {
	Name       string `json:"name" toml:"name"`
	URL        string `json:"url" toml:"url"`
	IsExternal bool   `json:"is_external,omitempty" toml:"is_external,omitempty"`
	IsActive   bool   `json:"is_active,omitempty" toml:"is_active,omitempty"`
}

var DefaultConfig = ContentConfig{
	ContentDir:     "content",
	StaticDirs:     []string{"static"},
	UploadDir:      "uploads",
	SidecarDB:      "sqlite.db",
	ThemeDir:       "tmpl",
	DefaultNewHint: "blog",
	Addr:           "0.0.0.0:8081",
}

var DefaultSiteConfig = SiteConfig{
	Author:      "Kalyan",
	AuthorEmail: "",

	Title: "Kalyan",
	Navigation: []NavigationLink{
		{
			Name: "Home",
			URL:  "/",
		},
		{
			Name: "About",
			URL:  "/about",
		},
	},
}
