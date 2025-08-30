package cmd

import (
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"oddity/pkg/config"
	"oddity/pkg/run"
)

var configPath string

var runCmd = &cobra.Command{
	Use:   "run",
	Short: "Run the oddity web server",
	Long:  `Start the oddity web server with content management and admin interface.`,
	Run:   runCmdExec,
}

func init() {
	rootCmd.AddCommand(runCmd)
	runCmd.Flags().StringVar(&configPath, "config", "", "Path to TOML config file")
}

func runCmdExec(cmd *cobra.Command, args []string) {

	var cfg config.Config
	var err error
	if configPath != "" {
		cfg, err = config.LoadConfigTOML(configPath)
		if err != nil {
			log.Fatalf("error loading config from %s: %v", configPath, err)
		}
	} else if _, err := os.Stat("config.toml"); err == nil {
		cfg, err = config.LoadConfigTOML("config.toml")
		if err != nil {
			log.Fatalf("error loading config from config.toml: %v", err)
		}
	} else {
		log.Warn("No config file specified and config.toml not found. Using default configuration.")
		cfg = config.NewDefaultConfig()
	}

	run.StartServer(cfg)
}
