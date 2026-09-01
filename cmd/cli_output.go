package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

const cliSchemaVersion = 1

const (
	exitSuccess     = 0
	exitUsage       = 2
	exitConfig      = 3
	exitDatabase    = 4
	exitSchema      = 5
	exitMaintenance = 6
)

type cliErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Field   string `json:"field,omitempty"`
}

type cliErrorOutput struct {
	SchemaVersion int            `json:"schema_version"`
	OK            bool           `json:"ok"`
	Error         cliErrorDetail `json:"error"`
}

type cliExitError struct {
	code     int
	reported bool
	err      error
}

func (e *cliExitError) Error() string {
	if e == nil || e.err == nil {
		return "command failed"
	}
	return e.err.Error()
}

func (e *cliExitError) Unwrap() error { return e.err }

func commandExitCode(err error) int {
	if err == nil {
		return exitSuccess
	}
	var exitErr *cliExitError
	if errors.As(err, &exitErr) && exitErr.code != 0 {
		return exitErr.code
	}
	return 1
}

func commandErrorReported(err error) bool {
	var exitErr *cliExitError
	return errors.As(err, &exitErr) && exitErr.reported
}

func writeJSON(writer io.Writer, value interface{}) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(value)
}

func failCommand(cmd *cobra.Command, jsonOutput bool, exitCode int, code, message, field string, cause error) error {
	if cause == nil {
		cause = errors.New(message)
	}
	reported := false
	if jsonOutput {
		reported = writeJSON(cmd.OutOrStdout(), cliErrorOutput{
			SchemaVersion: cliSchemaVersion,
			OK:            false,
			Error:         cliErrorDetail{Code: code, Message: message, Field: field},
		}) == nil
	} else {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", code, message)
		reported = true
	}
	return &cliExitError{code: exitCode, reported: reported, err: cause}
}

func usageError(message string) error {
	return &cliExitError{code: exitUsage, err: errors.New(message)}
}

func noCommandArgs(_ *cobra.Command, args []string) error {
	if len(args) == 0 {
		return nil
	}
	return usageError(fmt.Sprintf("unexpected command arguments: %v", args))
}

func noCommandArgsJSON(jsonOutput *bool) cobra.PositionalArgs {
	return func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return nil
		}
		enabled := jsonOutput != nil && *jsonOutput
		return failCommand(cmd, enabled, exitUsage, "USAGE_INVALID", "unexpected command arguments", "arguments", errors.New("unexpected command arguments"))
	}
}
