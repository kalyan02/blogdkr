package cmdutil

import (
	"fmt"
	"os"

	"github.com/sirupsen/logrus"

	"oddity/pkg/config"
	"oddity/tmpl"
)

func GenerateDefaultTemplates(configPath string, _ bool) {
	cfg, err := config.LoadConfigTOML(configPath)
	if err != nil {
		logrus.Fatalf("error loading config from %s: %v", configPath, err)
	}

	tmplDir := "tmpl"
	if cfg.Content.ThemeDir != "" {
		tmplDir = cfg.Content.ThemeDir
	}
	tmplDir, err = cfg.ResolveDir(tmplDir)
	if err != nil {
		logrus.Fatalf("Failed to resolve template directory: %v", err)
	}

	// if its not directory, error out
	info, err := os.Stat(tmplDir)
	if os.IsNotExist(err) {
		err = os.MkdirAll(tmplDir, os.ModePerm)
		if err != nil {
			logrus.Fatalf("Failed to create template directory %s: %v", tmplDir, err)
		}
	} else if err != nil {
		logrus.Fatalf("Failed to access template directory %s: %v", tmplDir, err)
	} else if !info.IsDir() {
		logrus.Fatalf("Template path %s is not a directory", tmplDir)
	}

	err = tmpl.Create(tmplDir)
	if err != nil {
		logrus.Fatalf("Failed to create template files in %s: %v", tmplDir, err)
	}

	fmt.Printf("Default template files created in %s\n", tmplDir)
}
