package cmd

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/willfong/load-generator/internal/ui"
)

//go:embed schemas/*.sql
var schemaFS embed.FS

// schemaCmd represents the schema command
var schemaCmd = &cobra.Command{
	Use:   "schema [type]",
	Short: "Output database schema files",
	Long: `Output the SQL schema for setting up the database.

Available schema types:
  full      Complete schema with tables and indexes (default)
  tables    Tables only, no indexes (for bulk loading)
  indexes   Indexes only (run after bulk data load)

The schema is designed for MariaDB 11.8+ but should work with MySQL 8+.

Bulk Loading Strategy:
  For optimal bulk loading performance:
  1. Create tables without indexes: loadgen schema tables | mysql ...
  2. Load data using LOAD DATA INFILE
  3. Create indexes: loadgen schema indexes | mysql ...

Examples:
  loadgen schema                        # Output complete schema
  loadgen schema full > schema.sql      # Save full schema to file
  loadgen schema tables | mysql -u root bank  # Create tables only
  loadgen schema indexes                # Output index creation SQL`,
	Args: cobra.MaximumNArgs(1),
	Run:  runSchema,
}

var schemaOutputFile string

func init() {
	rootCmd.AddCommand(schemaCmd)
	schemaCmd.Flags().StringVarP(&schemaOutputFile, "output", "o", "", "output file (default: stdout)")
}

func runSchema(cmd *cobra.Command, args []string) {
	u := ui.New()
	if noColor {
		u.SetNoColor(true)
	}

	schemaType := "full"
	if len(args) > 0 {
		schemaType = args[0]
	}

	var filename string
	switch schemaType {
	case "full":
		filename = "schemas/schema.sql"
	case "tables":
		filename = "schemas/schema_no_indexes.sql"
	case "indexes":
		filename = "schemas/schema_indexes.sql"
	default:
		fmt.Fprintln(os.Stderr, u.Error(fmt.Sprintf("Unknown schema type '%s'", schemaType)))
		fmt.Fprintln(os.Stderr, "Valid types: full, tables, indexes")
		os.Exit(1)
	}

	content, err := schemaFS.ReadFile(filename)
	if err != nil {
		fmt.Fprintln(os.Stderr, u.Error(fmt.Sprintf("Reading schema: %v", err)))
		os.Exit(1)
	}

	if schemaOutputFile != "" {
		// Ensure directory exists
		dir := filepath.Dir(schemaOutputFile)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				fmt.Fprintln(os.Stderr, u.Error(fmt.Sprintf("Creating directory: %v", err)))
				os.Exit(1)
			}
		}

		if err := os.WriteFile(schemaOutputFile, content, 0644); err != nil {
			fmt.Fprintln(os.Stderr, u.Error(fmt.Sprintf("Writing file: %v", err)))
			os.Exit(1)
		}
		fmt.Fprintln(os.Stderr, u.Success("Schema written to: "+schemaOutputFile))
	} else {
		fmt.Print(string(content))
	}
}
