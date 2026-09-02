package service

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/sirupsen/logrus"
)

func TestRedactLogLineCoversHeadersStructuredJSONSecretsAndIPAddresses(t *testing.T) {
	line := `Authorization: Bearer abc.def password=plain "access_token":"jwt-value" client_secret: 'client-value' lease_token=lease-value connection_token=connection-value nonce=nonce-value allocation_uuid=allocation-value session_uuid=session-value "route_leases":["route-one","route-two"] remote=203.0.113.42 peer=198.51.100.8:21116 ipv6=[2001:db8::42]:443 direct=2001:db8:1::9 safe=value`
	redacted := redactLogLine(line)
	for _, secret := range []string{"abc.def", "plain", "jwt-value", "client-value", "lease-value", "connection-value", "nonce-value", "allocation-value", "session-value", "route-one", "route-two", "203.0.113.42", "198.51.100.8", "2001:db8::42", "2001:db8:1::9"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("redacted line still contains %q: %s", secret, redacted)
		}
	}
	if !strings.Contains(redacted, "safe=value") || !strings.Contains(redacted, ":21116") || !strings.Contains(redacted, ":443") ||
		strings.Count(redacted, "[REDACTED]") != 10 || strings.Count(redacted, "[REDACTED_IP]") != 4 {
		t.Fatalf("unexpected redacted line: %s", redacted)
	}
}

func TestRedactLogLineLeavesInvalidIPLikeValuesUntouched(t *testing.T) {
	line := `version=1.1.16.999 timestamp=11:07:31 invalid=999.999.999.999`
	if redacted := redactLogLine(line); redacted != line {
		t.Fatalf("non-IP values changed: %q", redacted)
	}
}

func TestDetectLogLevelUsesTheMostSevereKnownMarker(t *testing.T) {
	if got := detectLogLevel("time=... level=error request failed"); got != "error" {
		t.Fatalf("detectLogLevel() = %q, want error", got)
	}
	if got := detectLogLevel("ordinary line"); got != "info" {
		t.Fatalf("detectLogLevel() = %q, want info", got)
	}
}

func TestOpenControlLogRejectsSymlinksAndNonRegularFiles(t *testing.T) {
	directory := t.TempDir()
	regular := filepath.Join(directory, "starry.log")
	if err := os.WriteFile(regular, []byte("safe\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	file, err := openControlLog(regular)
	if err != nil {
		t.Fatalf("regular log rejected: %v", err)
	}
	_ = file.Close()

	symlink := filepath.Join(directory, "relay.log")
	if err := os.Symlink(regular, symlink); err != nil {
		t.Fatal(err)
	}
	if file, err := openControlLog(symlink); err == nil {
		_ = file.Close()
		t.Fatal("symlink log source accepted")
	}
	if file, err := openControlLog(directory); err == nil {
		_ = file.Close()
		t.Fatal("directory log source accepted")
	}
}

func TestKessokuLoggerIsAnImplicitControlLogSource(t *testing.T) {
	securityAuditDatabase(t, true)
	directory := t.TempDir()
	logPath := filepath.Join(directory, "kessoku.log")
	if err := os.WriteFile(logPath, []byte("level=info msg=ready\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{
		Logger:        config.Logger{Path: logPath},
		ServerControl: config.ServerControl{Instances: []config.StarryInstance{{ID: "center", Name: "Center"}}},
	}
	service := NewStarryControlService(cfg, logrus.New(), nil)
	ctx := starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{ActorUserID: 1, RequestID: "logs-implicit-source"})
	sources, err := service.LogSources(ctx, "center")
	if err != nil {
		t.Fatal(err)
	}
	if len(sources) != 1 || sources[0].ID != "kessoku" || !sources[0].Available || !sources[0].LevelMutable {
		t.Fatalf("implicit sources = %+v", sources)
	}
	ctx = starrycontrol.WithRequestMetadata(context.Background(), starrycontrol.RequestMetadata{ActorUserID: 1, RequestID: "logs-implicit-read"})
	result, err := service.Logs(ctx, "center", "kessoku", 20)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Entries) != 1 || result.Entries[0].Text != "level=info msg=ready" {
		t.Fatalf("implicit Kessoku log result = %+v", result)
	}
}

func TestExplicitCrossDirectorySourcesDoNotExpandLoggerAccess(t *testing.T) {
	cfg := &config.Config{
		Logger: config.Logger{Path: filepath.Join(t.TempDir(), "kessoku.log")},
		ServerControl: config.ServerControl{
			LogDirectory: t.TempDir(),
			LogSources:   []config.ControlLogSource{{ID: "starry", Label: "Starry", Component: "starry", File: "starry.log"}},
		},
	}
	directory, sources := controlLogConfiguration(cfg)
	if directory != cfg.ServerControl.LogDirectory || len(sources) != 1 || sources[0].ID != "starry" {
		t.Fatalf("cross-directory logger widened allowlist: directory=%q sources=%+v", directory, sources)
	}
}
