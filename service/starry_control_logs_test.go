package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRedactLogLineCoversHeadersStructuredAndJSONSecrets(t *testing.T) {
	line := `Authorization: Bearer abc.def password=plain "access_token":"jwt-value" client_secret: 'client-value' safe=value`
	redacted := redactLogLine(line)
	for _, secret := range []string{"abc.def", "plain", "jwt-value", "client-value"} {
		if strings.Contains(redacted, secret) {
			t.Fatalf("redacted line still contains %q: %s", secret, redacted)
		}
	}
	if !strings.Contains(redacted, "safe=value") || strings.Count(redacted, "[REDACTED]") != 4 {
		t.Fatalf("unexpected redacted line: %s", redacted)
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
