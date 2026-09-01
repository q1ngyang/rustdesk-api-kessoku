package main

import (
	"errors"
	"fmt"
	"io"

	databaseSchema "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/database"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/spf13/cobra"
)

type maintenanceCommandOutput struct {
	SchemaVersion          int             `json:"schema_version"`
	Operation              string          `json:"operation"`
	Success                bool            `json:"success"`
	RequestID              string          `json:"request_id"`
	UserID                 uint            `json:"user_id,omitempty"`
	Username               string          `json:"username,omitempty"`
	AuthVersion            uint64          `json:"auth_version,omitempty"`
	PasswordReset          bool            `json:"password_reset"`
	TwoFactorReset         bool            `json:"two_factor_reset"`
	TwoFactorWasConfigured bool            `json:"two_factor_was_configured"`
	LoginChallengesCleared int64           `json:"login_challenges_cleared"`
	ScopesCleared          int64           `json:"scopes_cleared"`
	SessionsRevoked        int64           `json:"sessions_revoked"`
	Error                  *cliErrorDetail `json:"error,omitempty"`
}

func newMaintenanceCommand(configPath *string) *cobra.Command {
	command := &cobra.Command{Use: "maintenance", Short: "Run local-only, audited account recovery operations", Args: noCommandArgs}
	command.AddCommand(newRecoverAdminCommand(configPath), newResetTwoFactorCommand(configPath))
	return command
}

func newRecoverAdminCommand(configPath *string) *cobra.Command {
	var userID uint
	var username, confirmUsername, passwordFile string
	var resetTwoFactor, jsonOutput bool
	command := &cobra.Command{
		Use:   "recover-admin",
		Short: "Restore one confirmed local account as an enabled super administrator",
		Args:  noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := requestID()
			output := maintenanceCommandOutput{SchemaVersion: cliSchemaVersion, Operation: "recover_admin", RequestID: request}
			selector := service.MaintenanceSelector{UserID: userID, Username: username, ConfirmUsername: confirmUsername}
			if err := validateCLISelector(selector); err != nil {
				return writeMaintenanceFailure(cmd, jsonOutput, output, exitUsage, service.MaintenanceCodeSelectorInvalid, err.Error(), err)
			}
			password := ""
			if passwordFile != "" {
				var err error
				password, err = passwordFromFile(passwordFile)
				if err != nil {
					return writeMaintenanceFailure(cmd, jsonOutput, output, exitUsage, "PASSWORD_FILE_INVALID", "password file is invalid", err)
				}
			}
			path := configPathValue(configPath)
			closeDatabase, err := initializeMaintenance(cmd.Context(), path, maintenanceLogWriter(cmd, jsonOutput))
			if err != nil {
				code, exitCode, message := maintenanceInitializationError(err)
				return writeMaintenanceFailure(cmd, jsonOutput, output, exitCode, code, message, err)
			}
			defer closeDatabase()
			result, err := service.AllService.UserService.RecoverAdministratorContext(cmd.Context(), service.RecoverAdministratorOptions{
				Selector: selector, RequestID: request, Password: password, ResetTwoFactor: resetTwoFactor,
			})
			applyMaintenanceResult(&output, result)
			if err != nil {
				code := service.MaintenanceErrorCode(err, service.MaintenanceCodeRecoveryFailed)
				return writeMaintenanceFailure(cmd, jsonOutput, output, exitMaintenance, code, err.Error(), err)
			}
			output.Success = true
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), output)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "administrator recovered: user=%s id=%d auth_version=%d sessions_revoked=%d\n", output.Username, output.UserID, output.AuthVersion, output.SessionsRevoked)
			return err
		},
	}
	command.Flags().UintVar(&userID, "user-id", 0, "target user ID")
	command.Flags().StringVar(&username, "username", "", "exact target username")
	command.Flags().StringVar(&confirmUsername, "confirm-username", "", "exact stored username confirmation")
	command.Flags().StringVar(&passwordFile, "password-file", "", "owner-only regular file containing the replacement password")
	command.Flags().BoolVar(&resetTwoFactor, "reset-2fa", false, "also clear TOTP configuration")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}

func newResetTwoFactorCommand(configPath *string) *cobra.Command {
	var userID uint
	var username, confirmUsername string
	var jsonOutput bool
	command := &cobra.Command{
		Use:   "reset-2fa",
		Short: "Clear one confirmed user's TOTP state and revoke all sessions",
		Args:  noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			request := requestID()
			output := maintenanceCommandOutput{SchemaVersion: cliSchemaVersion, Operation: "reset_2fa", RequestID: request}
			selector := service.MaintenanceSelector{UserID: userID, Username: username, ConfirmUsername: confirmUsername}
			if err := validateCLISelector(selector); err != nil {
				return writeMaintenanceFailure(cmd, jsonOutput, output, exitUsage, service.MaintenanceCodeSelectorInvalid, err.Error(), err)
			}
			closeDatabase, err := initializeMaintenance(cmd.Context(), configPathValue(configPath), maintenanceLogWriter(cmd, jsonOutput))
			if err != nil {
				code, exitCode, message := maintenanceInitializationError(err)
				return writeMaintenanceFailure(cmd, jsonOutput, output, exitCode, code, message, err)
			}
			defer closeDatabase()
			result, err := service.AllService.UserService.ResetTwoFactorMaintenanceContext(cmd.Context(), service.ResetTwoFactorOptions{Selector: selector, RequestID: request})
			applyMaintenanceResult(&output, result)
			if err != nil {
				code := service.MaintenanceErrorCode(err, service.MaintenanceCodeTwoFactorResetFailed)
				return writeMaintenanceFailure(cmd, jsonOutput, output, exitMaintenance, code, err.Error(), err)
			}
			output.Success = true
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), output)
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "two-factor state reset: user=%s id=%d auth_version=%d sessions_revoked=%d\n", output.Username, output.UserID, output.AuthVersion, output.SessionsRevoked)
			return err
		},
	}
	command.Flags().UintVar(&userID, "user-id", 0, "target user ID")
	command.Flags().StringVar(&username, "username", "", "exact target username")
	command.Flags().StringVar(&confirmUsername, "confirm-username", "", "exact stored username confirmation")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}

func newResetAdminPasswordCommand(configPath *string) *cobra.Command {
	passwordFile := ""
	command := &cobra.Command{
		Use:     "reset-admin-pwd --password-file PATH",
		Example: "kessoku-api reset-admin-pwd --password-file /run/secrets/bootstrap-admin-password",
		Short:   "Reset Admin Password",
		Args:    noCommandArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			password, err := passwordFromFile(passwordFile)
			if err != nil {
				return fmt.Errorf("read password file: %w", err)
			}
			closeDatabase, err := initializeMaintenance(cmd.Context(), configPathValue(configPath), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			defer closeDatabase()
			admin := service.AllService.UserService.InfoById(1)
			if admin.Id == 0 {
				return errors.New("administrator user not found")
			}
			if err := service.AllService.UserService.UpdatePasswordContext(cmd.Context(), 0, requestID(), admin, password); err != nil {
				return fmt.Errorf("reset administrator password: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "reset password success!")
			return err
		},
	}
	command.Flags().StringVar(&passwordFile, "password-file", "", "owner-readable file containing the new password")
	_ = command.MarkFlagRequired("password-file")
	return command
}

func newResetUserPasswordCommand(configPath *string) *cobra.Command {
	var userID uint
	passwordFile := ""
	command := &cobra.Command{
		Use:     "reset-pwd --user-id ID --password-file PATH",
		Example: "kessoku-api reset-pwd --user-id 2 --password-file /run/secrets/user-password",
		Short:   "Reset User Password",
		Args:    noCommandArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if userID == 0 {
				return errors.New("user-id must be greater than 0")
			}
			password, err := passwordFromFile(passwordFile)
			if err != nil {
				return fmt.Errorf("read password file: %w", err)
			}
			closeDatabase, err := initializeMaintenance(cmd.Context(), configPathValue(configPath), cmd.ErrOrStderr())
			if err != nil {
				return err
			}
			defer closeDatabase()
			user := service.AllService.UserService.InfoById(userID)
			if user.Id == 0 {
				return errors.New("user not found")
			}
			if err := service.AllService.UserService.UpdatePasswordContext(cmd.Context(), 0, requestID(), user, password); err != nil {
				return fmt.Errorf("reset user password: %w", err)
			}
			_, err = fmt.Fprintln(cmd.OutOrStdout(), "reset password success!")
			return err
		},
	}
	command.Flags().UintVar(&userID, "user-id", 0, "user ID")
	command.Flags().StringVar(&passwordFile, "password-file", "", "owner-readable file containing the new password")
	_ = command.MarkFlagRequired("user-id")
	_ = command.MarkFlagRequired("password-file")
	return command
}

func validateCLISelector(selector service.MaintenanceSelector) error {
	if (selector.UserID == 0) == (selector.Username == "") {
		return errors.New("exactly one of --user-id or --username is required")
	}
	if selector.ConfirmUsername == "" {
		return errors.New("--confirm-username is required")
	}
	return nil
}

func configPathValue(value *string) string {
	if value == nil || *value == "" {
		return "./conf/config.yaml"
	}
	return *value
}

func maintenanceLogWriter(cmd *cobra.Command, jsonOutput bool) io.Writer {
	if jsonOutput {
		return io.Discard
	}
	return cmd.ErrOrStderr()
}

func maintenanceInitializationError(err error) (string, int, string) {
	if errors.Is(err, databaseSchema.ErrSchemaMismatch) {
		return service.MaintenanceCodeSchemaMismatch, exitSchema, "database schema must exactly match this binary"
	}
	return "MAINTENANCE_INITIALIZATION_FAILED", exitDatabase, "maintenance initialization failed"
}

func applyMaintenanceResult(output *maintenanceCommandOutput, result service.MaintenanceResult) {
	output.RequestID = result.RequestID
	output.UserID = result.UserID
	output.Username = result.Username
	output.AuthVersion = result.AuthVersion
	output.PasswordReset = result.PasswordReset
	output.TwoFactorReset = result.TwoFactorReset
	output.TwoFactorWasConfigured = result.TwoFactorWasConfigured
	output.LoginChallengesCleared = result.LoginChallengesCleared
	output.ScopesCleared = result.ScopesCleared
	output.SessionsRevoked = result.SessionsRevoked
}

func writeMaintenanceFailure(cmd *cobra.Command, jsonOutput bool, output maintenanceCommandOutput, exitCode int, code, message string, cause error) error {
	output.Error = &cliErrorDetail{Code: code, Message: message}
	if jsonOutput {
		_ = writeJSON(cmd.OutOrStdout(), output)
	} else {
		_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", code, message)
	}
	return &cliExitError{code: exitCode, reported: true, err: cause}
}
