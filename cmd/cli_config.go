package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	internalAuth "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/auth"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/controlauth"
	starryProvider "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol/starry"
	"github.com/spf13/cobra"
)

type configValidationIssue struct {
	Field   string `json:"field"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

type configValidationOutput struct {
	SchemaVersion int                     `json:"schema_version"`
	Valid         bool                    `json:"valid"`
	ConfigPath    string                  `json:"config_path"`
	DatabaseType  string                  `json:"database_type,omitempty"`
	Errors        []configValidationIssue `json:"errors"`
	Warnings      []configValidationIssue `json:"warnings"`
}

func newConfigCommand(configPath *string) *cobra.Command {
	command := &cobra.Command{Use: "config", Short: "Inspect Kessoku configuration", Args: noCommandArgs}
	command.AddCommand(newConfigValidateCommand(configPath))
	return command
}

func newConfigValidateCommand(configPath *string) *cobra.Command {
	jsonOutput := false
	command := &cobra.Command{
		Use:   "validate",
		Short: "Validate configuration without connecting or changing local state",
		Args:  noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			path := config.DefaultConfig
			if configPath != nil && *configPath != "" {
				path = *configPath
			}
			result := configValidationOutput{
				SchemaVersion: cliSchemaVersion,
				ConfigPath:    path,
				Errors:        []configValidationIssue{},
				Warnings:      []configValidationIssue{},
			}
			cfg := config.Config{}
			if _, err := config.Load(&cfg, path); err != nil {
				result.Errors = append(result.Errors, issueFromConfigLoadError(err))
				if jsonOutput {
					if writeErr := writeJSON(cmd.OutOrStdout(), result); writeErr != nil {
						return writeErr
					}
				} else {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "configuration invalid: %s\n", result.Errors[0].Message)
				}
				return &cliExitError{code: exitConfig, reported: true, err: err}
			}
			result.DatabaseType = cfg.DatabaseType()
			errorsFound, warnings := validateConfigurationReferences(&cfg)
			result.Errors = append(result.Errors, errorsFound...)
			result.Warnings = append(result.Warnings, warnings...)
			result.Valid = len(result.Errors) == 0
			if jsonOutput {
				if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			} else if result.Valid {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "configuration valid: %s (database: %s)\n", path, result.DatabaseType)
				for _, warning := range result.Warnings {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "warning %s: %s\n", warning.Field, warning.Message)
				}
			} else {
				for _, validationErr := range result.Errors {
					_, _ = fmt.Fprintf(cmd.ErrOrStderr(), "%s: %s\n", validationErr.Field, validationErr.Message)
				}
			}
			if !result.Valid {
				return &cliExitError{code: exitConfig, reported: true, err: errors.New("configuration is invalid")}
			}
			return nil
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}

func issueFromConfigLoadError(err error) configValidationIssue {
	message := ""
	if err != nil {
		message = err.Error()
	}
	field := "configuration"
	for _, candidate := range []string{
		"gorm.type", "gorm.max-idle-conns", "gorm.max-open-conns",
		"mysql.addr", "mysql.username", "mysql.dbname", "mysql.tls",
		"postgresql.host", "postgresql.user", "postgresql.dbname", "postgresql.port", "postgresql.sslmode",
		"gin.api-addr", "gin.mode", "gin.resources-path",
		"admin.id-server-port", "admin.relay-server-port", "admin.hello-file",
		"rustdesk.id-server", "rustdesk.relay-server", "rustdesk.api-server", "rustdesk.key", "rustdesk.key-file",
		"logger.path", "cache.type", "cache.file-dir", "cache.redis-addr",
		"two-factor", "server-control", "web-client", "media", "ldap", "auth",
	} {
		if strings.Contains(message, candidate) {
			field = candidate
			break
		}
	}
	return issueFromError(field, "CONFIG_INVALID", err)
}

func issueFromError(field, code string, err error) configValidationIssue {
	message := "configuration validation failed"
	if err != nil {
		message = err.Error()
	}
	return configValidationIssue{Field: field, Code: code, Message: message}
}

func validateConfigurationReferences(cfg *config.Config) (validationErrors, warnings []configValidationIssue) {
	addError := func(field, code string, err error) {
		validationErrors = append(validationErrors, issueFromError(field, code, err))
	}
	addWarning := func(field, code, message string) {
		warnings = append(warnings, configValidationIssue{Field: field, Code: code, Message: message})
	}
	if cfg == nil {
		addError("configuration", "CONFIG_INVALID", errors.New("configuration is unavailable"))
		return validationErrors, warnings
	}

	for _, item := range []struct{ field, path string }{
		{field: "gin.resources-path", path: cfg.Gin.ResourcesPath},
		{field: "admin.hello-file", path: cfg.Admin.HelloFile},
	} {
		field, reference := item.field, item.path
		if reference == "" {
			continue
		}
		if err := requireRegularReference(reference, false); err != nil {
			if field == "gin.resources-path" {
				if info, statErr := os.Stat(reference); statErr == nil && info.IsDir() {
					continue
				}
			}
			addError(field, "FILE_REFERENCE_INVALID", err)
		}
	}
	if cfg.Rustdesk.Key == "" && cfg.Rustdesk.KeyFile != "" {
		if err := requireNonemptyRegularReference(cfg.Rustdesk.KeyFile, false); err != nil {
			addError("rustdesk.key-file", "KEY_REFERENCE_INVALID", err)
		}
	}

	manager, authErr := internalAuth.NewManager(cfg.Auth)
	if authErr != nil {
		addError("auth", "AUTH_KEY_INVALID", authErr)
	}
	if cfg.Auth.Enabled {
		if err := requireRegularReference(cfg.Auth.CurrentKey.PrivateKeyFile, true); err != nil {
			addError("auth.current-key.private-key-file", "PRIVATE_KEY_REFERENCE_INVALID", err)
		}
		for index, previous := range cfg.Auth.PreviousKeys {
			if err := requireRegularReference(previous.PublicKeyFile, false); err != nil {
				addError(fmt.Sprintf("auth.previous-keys[%d].public-key-file", index), "PUBLIC_KEY_REFERENCE_INVALID", err)
			}
		}
	}
	if cfg.Auth.Internal.Enabled {
		if err := requireRegularReference(cfg.Auth.Internal.ServerKeyFile, true); err != nil {
			addError("auth.internal.server-key-file", "PRIVATE_KEY_REFERENCE_INVALID", err)
		}
		if _, err := tls.LoadX509KeyPair(cfg.Auth.Internal.ServerCertFile, cfg.Auth.Internal.ServerKeyFile); err != nil {
			addError("auth.internal.server-cert-file", "TLS_KEYPAIR_INVALID", err)
		}
		if err := validateCertificateBundle(cfg.Auth.Internal.ClientCAFile); err != nil {
			addError("auth.internal.client-ca-file", "CERTIFICATE_BUNDLE_INVALID", err)
		}
	}

	if cfg.Mysql.CaFile != "" && cfg.DatabaseType() == config.TypeMysql {
		if err := validateCertificateBundle(cfg.Mysql.CaFile); err != nil {
			addError("mysql.ca-file", "CERTIFICATE_BUNDLE_INVALID", err)
		}
	}
	if cfg.Postgresql.Sslrootcert != "" && cfg.DatabaseType() == config.TypePostgresql {
		if err := validateCertificateBundle(cfg.Postgresql.Sslrootcert); err != nil {
			addError("postgresql.ssl-root-cert", "CERTIFICATE_BUNDLE_INVALID", err)
		}
	}
	if cfg.Ldap.Enable && cfg.Ldap.TlsCaFile != "" {
		if err := validateCertificateBundle(cfg.Ldap.TlsCaFile); err != nil {
			addError("ldap.tls-ca-file", "CERTIFICATE_BUNDLE_INVALID", err)
		}
	}

	for index, instance := range cfg.ServerControl.Instances {
		if !instance.Enabled {
			continue
		}
		prefix := fmt.Sprintf("server-control.instances[%d]", index)
		for _, item := range []struct{ field, path string }{
			{field: "client-key-file", path: instance.ClientKeyFile},
			{field: "control-key-file", path: instance.ControlKeyFile},
		} {
			field, path := item.field, item.path
			if err := requireRegularReference(path, true); err != nil {
				addError(prefix+"."+field, "PRIVATE_KEY_REFERENCE_INVALID", err)
			}
		}
		if manager != nil {
			fingerprint, err := controlauth.PrivateKeyPublicFingerprint(instance.ControlKeyFile)
			if err != nil {
				addError(prefix+".control-key-file", "CONTROL_KEY_INVALID", err)
			} else if manager.UsesPublicKeyFingerprint(fingerprint) {
				addError(prefix+".control-key-file", "CONTROL_KEYRING_NOT_ISOLATED", errors.New("control and access-token keyrings must use different keys"))
			}
		}
		if _, err := starryProvider.NewProvider(instance, cfg.ServerControl); err != nil {
			addError(prefix, "STARRY_INSTANCE_INVALID", err)
		}
	}

	if cfg.TwoFactor.Enabled {
		info, err := os.Lstat(cfg.TwoFactor.KeyFile)
		if errors.Is(err, os.ErrNotExist) {
			addWarning("two-factor.key-file", "TWO_FACTOR_KEY_PENDING", "key does not exist and will be created with mode 0600 by full service startup")
		} else if err != nil {
			addError("two-factor.key-file", "TWO_FACTOR_KEY_INVALID", err)
		} else if !info.Mode().IsRegular() || info.Mode().Perm()&0o077 != 0 {
			addError("two-factor.key-file", "TWO_FACTOR_KEY_INVALID", errors.New("two-factor key must be a regular owner-only file"))
		} else if contents, readErr := os.ReadFile(cfg.TwoFactor.KeyFile); readErr != nil || len(contents) != 32 {
			if readErr == nil {
				readErr = errors.New("two-factor key must contain exactly 32 bytes")
			}
			addError("two-factor.key-file", "TWO_FACTOR_KEY_INVALID", readErr)
		}
	}
	if len(cfg.ServerControl.LogSources) > 0 {
		if info, err := os.Stat(cfg.ServerControl.LogDirectory); err != nil || !info.IsDir() {
			if err == nil {
				err = errors.New("log directory is not a directory")
			}
			addError("server-control.log-directory", "FILE_REFERENCE_INVALID", err)
		}
		for index, source := range cfg.ServerControl.LogSources {
			path := filepath.Join(cfg.ServerControl.LogDirectory, source.File)
			if err := requireRegularReference(path, false); err != nil {
				addWarning(fmt.Sprintf("server-control.log-sources[%d].file", index), "LOG_SOURCE_UNAVAILABLE", err.Error())
			}
		}
	}
	return validationErrors, warnings
}

func requireRegularReference(path string, ownerOnly bool) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("file path is required")
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("reference must be a regular file")
	}
	if ownerOnly && info.Mode().Perm()&0o077 != 0 {
		return errors.New("private file must not be accessible by group or other users")
	}
	return nil
}

func requireNonemptyRegularReference(path string, ownerOnly bool) error {
	if err := requireRegularReference(path, ownerOnly); err != nil {
		return err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if strings.TrimSpace(string(contents)) == "" {
		return errors.New("referenced file is empty")
	}
	return nil
}

func validateCertificateBundle(path string) error {
	if err := requireRegularReference(path, false); err != nil {
		return err
	}
	contents, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	pool := x509.NewCertPool()
	if !pool.AppendCertsFromPEM(contents) {
		return errors.New("file contains no valid PEM certificates")
	}
	return nil
}
