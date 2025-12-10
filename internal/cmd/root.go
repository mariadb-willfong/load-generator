package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var verbose bool
var noColor bool

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "loadgen",
	Short: "Bank-in-a-Box load generator for database testing",
	Long: `A realistic banking data generator and load simulator.

This tool generates synthetic but realistic banking data and simulates
concurrent customer interactions for database load testing.

Phase 1 (generate): Create bulk historical data for database seeding
Phase 2 (simulate): Run live customer interaction simulations

Tunable parameters are in internal/config/defaults.go - edit and recompile.

Example usage:
  loadgen generate --customers 100000 --years 5
  loadgen simulate --concurrency 1000 --db "user:pass@tcp(host:3306)/bank"`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&noColor, "no-color", false, "disable colors and animations")

	// Silence usage on error - we'll print our own messages
	rootCmd.SilenceUsage = true

	// Set version template
	rootCmd.SetVersionTemplate("{{.Version}}\n")
}

// Verbose returns whether verbose mode is enabled
func Verbose() bool {
	return verbose
}

// Exit with code
func Exit(code int) {
	os.Exit(code)
}
