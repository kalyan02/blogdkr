package cmd

import (
	"github.com/spf13/cobra"

	"oddity/pkg/cmdutil"
)

var setupForce bool

var setupCmd = &cobra.Command{
	Use:   "setup",
	Short: "Setup commands",
	Long:  `Commands for managing admin users and system administration.`,
}

var adminAuthCmd = &cobra.Command{
	Use:   "auth",
	Short: "Set up admin username and password",
	Long:  `Interactive command to set up or update admin username and password.`,
	Run:   func(cmd *cobra.Command, args []string) { cmdutil.RunAdminAuthSetup(configPath) },
}

var newConfigCmd = &cobra.Command{
	Use:   "config",
	Short: "Generate default configuration file",
	Long:  `Generates a default configuration file named config.yaml in the current directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdutil.GenerateDefault(configPath, setupForce)
	},
}

var newTmplCmd = &cobra.Command{
	Use:   "tmpl",
	Short: "Generate default template files",
	Long:  `Generates default HTML template files in the specified directory.`,
	Run: func(cmd *cobra.Command, args []string) {
		cmdutil.GenerateDefaultTemplates(configPath, setupForce)
	},
}

func init() {
	rootCmd.AddCommand(setupCmd)
	setupCmd.AddCommand(adminAuthCmd)
	setupCmd.AddCommand(newConfigCmd)
	setupCmd.AddCommand(newTmplCmd)
	// runCmd.Flags().StringVar(&configPath, "config", "", "Path to TOML config file")
	setupCmd.PersistentFlags().StringVar(&configPath, "config", "config.toml", "Path to TOML config file")
	// force
	setupCmd.PersistentFlags().BoolVar(&setupForce, "force", false, "Force overwrite of existing files")
}
