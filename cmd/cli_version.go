package main

import (
	"fmt"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/buildinfo"
	"github.com/spf13/cobra"
)

type versionOutput struct {
	SchemaVersion int `json:"schema_version"`
	buildinfo.Details
}

func newVersionCommand() *cobra.Command {
	jsonOutput := false
	command := &cobra.Command{
		Use:   "version",
		Short: "Print Kessoku binary and database compatibility information",
		Args:  noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			details := buildinfo.Current()
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), versionOutput{SchemaVersion: cliSchemaVersion, Details: details})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "component: %s\nversion: %s\ndatabase schema: %d\ngit commit: %s\nbuild time: %s\ngo version: %s\n",
				details.Component, details.Version, details.DatabaseSchema, details.GitCommit, details.BuildTime, details.GoVersion)
			return err
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}
