package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/willfong/load-generator/internal/ui"
)

// Version information - set at build time via ldflags
var (
	Version   = "dev"
	GitCommit = "none"
	BuildDate = "unknown"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		u := ui.New()
		if noColor {
			u.SetNoColor(true)
		}

		fmt.Println(u.Header("Bank-in-a-Box Load Generator"))
		fmt.Println()
		fmt.Println(u.KeyValue("Version", Version))
		fmt.Println(u.KeyValue("Git Commit", GitCommit))
		fmt.Println(u.KeyValue("Built", BuildDate))
		fmt.Println(u.KeyValue("Go Version", runtime.Version()))
		fmt.Println(u.KeyValue("OS/Arch", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)))
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
