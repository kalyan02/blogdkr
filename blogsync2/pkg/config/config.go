package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Dropbox   DropboxConfig `toml:"dropbox"`
	Server    ServerConfig  `toml:"server"`
	Sync      SyncConfig    `toml:"sync"`
	Build     BuildConfig   `toml:"build"`
	Database  DatabaseConfig `toml:"database"`
	CopyRules []CopyRule    `toml:"copy_rules"`
}

type DropboxConfig struct {
	AppKey      string `toml:"app_key"`
	AppSecret   string `toml:"app_secret"`
	RedirectURI string `toml:"redirect_uri"`
}

type ServerConfig struct {
	Host        string `toml:"host"`
	Port        int    `toml:"port"`
	AdminPort   int    `toml:"admin_port"`
	WebhookPath string `toml:"webhook_path"`
}

type SyncConfig struct {
	LocalBasePath string `toml:"local_base_path"`
	DropboxFolder string `toml:"dropbox_folder"`
}

type BuildConfig struct {
	Command          string `toml:"command"`
	WorkingDirectory string `toml:"working_directory"`
}

type DatabaseConfig struct {
	Path string `toml:"path"`
}

type CopyRule struct {
	SourcePattern string `toml:"source_pattern"`
	Destination   string `toml:"destination"`
	Recursive     *bool  `toml:"recursive,omitempty"`
}

func Load(path string) (*Config, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil, err
	}

	var config Config
	_, err := toml.DecodeFile(path, &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}

func Save(config *Config, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	encoder := toml.NewEncoder(file)
	return encoder.Encode(config)
}

func Default() *Config {
	recursive := true
	return &Config{
		Dropbox: DropboxConfig{
			AppKey:      "your_app_key",
			AppSecret:   "your_app_secret",
			RedirectURI: "http://localhost:3000/auth/callback",
		},
		Server: ServerConfig{
			Host:        "0.0.0.0",
			Port:        3000,
			AdminPort:   3001,
			WebhookPath: "/webhook",
		},
		Sync: SyncConfig{
			LocalBasePath: "./sync",
			DropboxFolder: "/",
		},
		Build: BuildConfig{
			Command:          "zola build",
			WorkingDirectory: "./sync",
		},
		Database: DatabaseConfig{
			Path: "./database.db",
		},
		CopyRules: []CopyRule{
			{
				SourcePattern: "./sync/public/**/*",
				Destination:   "./output",
				Recursive:     &recursive,
			},
		},
	}
}