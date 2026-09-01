package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
	appHTTP "github.com/q1ngyang/rustdesk-api-kessoku/v3/http"
	databaseSchema "github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/database"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

func newRootCommand() *cobra.Command {
	configPath := config.DefaultConfig
	root := &cobra.Command{
		Use:           "kessoku-api",
		Short:         "Kessoku control plane for RustDesk deployments",
		SilenceErrors: true,
		SilenceUsage:  true,
		Args:          noCommandArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			global.ConfigPath = configPath
			InitGlobal()
			global.Logger.Info("API SERVER START")
			httpApiInit()
			return nil
		},
	}
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return usageError(err.Error())
	})
	root.PersistentFlags().StringVarP(&configPath, "config", "c", config.DefaultConfig, "choose config file")
	root.AddCommand(
		newVersionCommand(),
		newConfigCommand(&configPath),
		newDatabaseCommand(&configPath),
		newMaintenanceCommand(&configPath),
		newResetAdminPasswordCommand(&configPath),
		newResetUserPasswordCommand(&configPath),
	)
	return root
}

// Kept as a small seam so the root command remains testable without starting
// listeners. Production calls the repository HTTP initializer directly.
var httpApiInit = appHTTP.ApiInit

var openPasswordFileNoFollow = openPasswordFileSecure

func requestID() string {
	value, err := uuid.NewV7()
	if err != nil {
		value = uuid.New()
	}
	return value.String()
}

func passwordFromFile(path string) (string, error) {
	if strings.TrimSpace(path) == "" {
		return "", errors.New("password-file is required")
	}
	pathInfo, err := os.Lstat(path)
	if err != nil {
		return "", err
	}
	if !pathInfo.Mode().IsRegular() {
		return "", errors.New("password-file must be a regular file")
	}
	if pathInfo.Mode().Perm()&0o077 != 0 {
		return "", errors.New("password-file must not be accessible by group or other users")
	}
	file, err := openPasswordFileNoFollow(path)
	if err != nil {
		return "", fmt.Errorf("open password-file without following symlinks: %w", err)
	}
	if file == nil {
		return "", errors.New("open password-file")
	}
	defer file.Close()
	fileInfo, err := file.Stat()
	if err != nil {
		return "", err
	}
	if !fileInfo.Mode().IsRegular() || !os.SameFile(pathInfo, fileInfo) {
		return "", errors.New("password-file changed while opening")
	}
	if fileInfo.Mode().Perm()&0o077 != 0 {
		return "", errors.New("password-file must not be accessible by group or other users")
	}
	contents, err := io.ReadAll(io.LimitReader(file, 131))
	if err != nil {
		return "", err
	}
	if len(contents) > 130 {
		return "", errors.New("password must contain 12 to 128 bytes")
	}
	password := strings.TrimSuffix(strings.TrimSuffix(string(contents), "\n"), "\r")
	if len(password) < 12 || len(password) > 128 {
		return "", errors.New("password must contain 12 to 128 bytes")
	}
	return password, nil
}

func initializeMaintenance(ctx context.Context, configPath string, loggerWriter io.Writer) (func(), error) {
	cfg := &config.Config{}
	viperConfig, err := config.Load(cfg, configPath)
	if err != nil {
		return func() {}, err
	}
	logger := commandLoggerForWriter(loggerWriter)
	db, closeDatabase, err := openConfiguredDatabase(ctx, cfg, databaseReadWrite, logger)
	if err != nil {
		return func() {}, err
	}
	if _, err := databaseSchema.RequireCurrentSchema(db); err != nil {
		closeDatabase()
		return func() {}, err
	}
	global.ConfigPath = configPath
	global.Config = *cfg
	global.Viper = viperConfig
	global.Logger = logger
	global.DB = db
	service.NewMaintenance(&global.Config, global.DB, global.Logger)
	return closeDatabase, nil
}

func commandLoggerForWriter(writer io.Writer) *logrus.Logger {
	logger := logrus.New()
	logger.SetFormatter(&logrus.TextFormatter{DisableTimestamp: true, DisableColors: true})
	logger.SetOutput(writer)
	return logger
}
