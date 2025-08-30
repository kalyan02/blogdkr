package config

import (
	"bytes"
	"fmt"
	"os"

	toml "github.com/pelletier/go-toml/v2"

	log "github.com/sirupsen/logrus"
)

type Config struct {
	Content ContentConfig `toml:"content"`
	Site    SiteConfig    `toml:"site"`
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

func GenerateDefault() {
	defaultConfig := NewDefaultConfig()
	configData, err := defaultConfig.EncodeTOML()
	if err != nil {
		log.Fatalf("Failed to marshal default config: %v", err)
	}

	configFilePath := "config.toml"
	if _, err := os.Stat(configFilePath); err == nil {
		log.Fatalf("Config file %s already exists. Aborting to prevent overwrite.", configFilePath)
	}

	err = os.WriteFile(configFilePath, configData, 0644)
	if err != nil {
		log.Fatalf("Failed to write config file: %v", err)
	}

	fmt.Printf("Default configuration file created at %s\n", configFilePath)
}

func LoadConfigTOML(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("failed to read config file: %w", err)
	}
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("failed to unmarshal config file: %w", err)
	}
	return cfg, nil
}

type ContentConfig struct {
	ContentDir     string   `toml:"content_dir"`
	StaticDirs     []string `toml:"static_dirs"`
	ThemeDir       string   `toml:"theme_dir,omitempty"`
	Addr           string   `toml:"addr"`
	DefaultNewHint string   `toml:"default_new_hint"`
	SidecarDB      string   `toml:"sidecar_db"`
	AdminAddr      string   `toml:"admin_addr,omitempty"`
}

var DefaultConfig = ContentConfig{
	ContentDir:     "content/content",
	StaticDirs:     []string{"content/static"},
	SidecarDB:      "content/sqlite.db",
	DefaultNewHint: "blog",
	Addr:           ":8081",
	AdminAddr:      ":8082",
}

type SiteConfig struct {
	Title       string           `toml:"title"`
	Description string           `toml:"description,omitempty"`
	BaseURL     string           `toml:"base_url,omitempty"`
	Navigation  []NavigationLink `toml:"navigation,inline"`
}

type NavigationLink struct {
	Name       string `json:"name" toml:"name"`
	URL        string `json:"url" toml:"url"`
	IsExternal bool   `json:"is_external,omitempty" toml:"is_external,omitempty"`
	IsActive   bool   `json:"is_active,omitempty" toml:"is_active,omitempty"`
}

var DefaultSiteConfig = SiteConfig{
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
