package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/devopsext/tools/common"
	"github.com/spf13/cobra"
)

var VERSION = "unknown"

var stdoutOptions = common.StdoutOptions{

	Format:          "text",
	Level:           "info",
	Template:        "{{.file}} {{.msg}}",
	TimestampFormat: time.RFC3339Nano,
	TextColors:      true,
	Debug:           false,
}

var stdout *common.Stdout

func Execute() {

	rootCmd := &cobra.Command{
		Use:   "tools",
		Short: "Tools",
		PersistentPreRun: func(cmd *cobra.Command, args []string) {

			stdoutOptions.Version = VERSION
			stdout = common.NewStdout(stdoutOptions)
			stdout.SetCallerOffset(1)
			stdout.Info("Booting...")
		},
		Run: func(cmd *cobra.Command, args []string) {

			stdout.Info("Log message...")

		},
	}

	flags := rootCmd.PersistentFlags()

	flags.StringVar(&stdoutOptions.Format, "stdout-format", stdoutOptions.Format, "Stdout format: json, text, template")
	flags.StringVar(&stdoutOptions.Level, "stdout-level", stdoutOptions.Level, "Stdout level: info, warn, error, debug, panic")
	flags.StringVar(&stdoutOptions.Template, "stdout-template", stdoutOptions.Template, "Stdout template")
	flags.StringVar(&stdoutOptions.TimestampFormat, "stdout-timestamp-format", stdoutOptions.TimestampFormat, "Stdout timestamp format")
	flags.BoolVar(&stdoutOptions.TextColors, "stdout-text-colors", stdoutOptions.TextColors, "Stdout text colors")
	flags.BoolVar(&stdoutOptions.Debug, "stdout-debug", stdoutOptions.Debug, "Stdout debug")

	rootCmd.AddCommand(&cobra.Command{
		Use:   "version",
		Short: "Print the version number",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(VERSION)
		},
	})

	if err := rootCmd.Execute(); err != nil {
		stdout.Error(err)
		os.Exit(1)
	}
}
