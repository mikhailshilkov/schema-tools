package cmd

import (
	"github.com/mikhailshilkov/schema-tools/cmd/compare"
	"github.com/mikhailshilkov/schema-tools/cmd/stats"
	"github.com/mikhailshilkov/schema-tools/cmd/version"
	"github.com/spf13/cobra"
)

func RootCmd() *cobra.Command {

	rootCmd := &cobra.Command{
		Use:   "schema-tools",
		Short: "Tool to be able to compare Pulumi schema.json files to understand changes",
	}

	rootCmd.AddCommand(stats.Command())
	rootCmd.AddCommand(version.Command())
	rootCmd.AddCommand(compare.Command())

	return rootCmd
}
