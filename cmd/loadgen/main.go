package main

import (
	"os"

	"github.com/willfong/load-generator/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
