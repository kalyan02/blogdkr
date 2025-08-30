package cmdutil

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/sirupsen/logrus"

	"oddity/pkg/config"
	"oddity/pkg/utils"
)

func GenerateDefault(configFilePath string, setupForce bool) {
	// if configPath is given and is not empty, use that path
	if filepath.Ext(configFilePath) != ".toml" {
		logrus.Fatalf("Config file must have .toml extension")
	}

	if configFilePath != "" {
		err := os.MkdirAll(filepath.Dir(configFilePath), 0755)
		if err != nil {
			logrus.Fatalf("Failed to create directories for config path %s: %v", configFilePath, err)
		}
	} else {
		configFilePath = "config.toml"
	}

	// print pwd
	pwd, err := os.Getwd()
	if err != nil {
		logrus.Fatalf("Failed to get current directory: %v", err)
	}
	fmt.Printf("Generating default configuration in %s\n", pwd)

	defaultConfig := config.NewDefaultConfig()
	configData, err := defaultConfig.EncodeTOML()
	if err != nil {
		logrus.Fatalf("Failed to marshal default config: %v", err)
	}

	if _, err := os.Stat(configFilePath); err == nil && !setupForce {
		logrus.Fatalf("Config file %s already exists. Aborting to prevent overwrite.", configFilePath)
	}

	err = os.WriteFile(configFilePath, configData, 0644)
	if err != nil {
		logrus.Fatalf("Failed to write config file: %v", err)
	}

	fmt.Printf("Default configuration file created at %s\n", configFilePath)

	// now ensure folders exist
	dirs := []string{defaultConfig.Content.ContentDir}
	dirs = append(dirs, defaultConfig.Content.StaticDirs...)
	if defaultConfig.Content.UploadDir != "" {
		dirs = append(dirs, defaultConfig.Content.UploadDir)
	}
	logrus.Infof("Ensuring content directories exist: %v", dirs)
	for _, dir := range dirs {
		relDir, err := utils.ResolveRelative(dir, configFilePath)
		if err != nil {
			logrus.Fatalf("Failed to resolve directory %s: %v", relDir, err)
		}

		if _, err := os.Stat(relDir); os.IsNotExist(err) {
			err := os.MkdirAll(relDir, 0755)
			if err != nil {
				logrus.Fatalf("Failed to create directory %s: %v", relDir, err)
			}
			fmt.Printf("Created directory: %s\n", relDir)
		} else if err != nil {
			logrus.Fatalf("Failed to access directory %s: %v", relDir, err)
		} else {
			logrus.Infof("Directory %s already exists, skipping", dir)
		}
	}

}
