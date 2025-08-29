package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "oddity",
	Short: "Oddity is a content management system",
	Long:  `A content management system built with Go that handles markdown content and provides a web interface for editing.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
}
