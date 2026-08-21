package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimePackagingExcludesBundledBrowserClients(t *testing.T) {
	destination := filepath.Join(t.TempDir(), "resources")
	command := exec.Command("sh", "scripts/copy-runtime-resources.sh", destination, "resources")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("copy runtime resources: %v: %s", err, output)
	}
	for _, required := range []string{"i18n", "public", "templates", "version"} {
		if _, err := os.Stat(filepath.Join(destination, required)); err != nil {
			t.Fatalf("runtime resource %q missing: %v", required, err)
		}
	}
	for _, forbidden := range []string{"web", "web2"} {
		if _, err := os.Stat(filepath.Join(destination, forbidden)); !os.IsNotExist(err) {
			t.Fatalf("bundled browser-client directory %q entered runtime package", forbidden)
		}
	}
}

func TestBuildDefinitionsCannotCopyAllResourceDirectories(t *testing.T) {
	files := []string{"Dockerfile", "Dockerfile.dev", "build.sh", "build.bat", ".github/workflows/build.yml"}
	for _, path := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		for _, forbidden := range []string{"cp -ar resources release/", "/app/resources /app/resources/", `xcopy resources release\resources`} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s contains unsafe broad resource copy %q", path, forbidden)
			}
		}
	}
	dockerIgnore, err := os.ReadFile(".dockerignore")
	if err != nil {
		t.Fatal(err)
	}
	for _, excluded := range []string{"resources/web/", "resources/web2/"} {
		if !strings.Contains(string(dockerIgnore), excluded) {
			t.Fatalf("Docker build context does not exclude %s", excluded)
		}
	}
}

func TestLocalReleaseBuildsRequireReviewedAdminAssets(t *testing.T) {
	linuxBuild, err := os.ReadFile("build.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(linuxBuild), "resources resources require-admin") {
		t.Fatal("Linux release build does not require reviewed admin assets")
	}
	windowsBuild, err := os.ReadFile("build.bat")
	if err != nil {
		t.Fatal(err)
	}
	windowsText := string(windowsBuild)
	if !strings.Contains(windowsText, `if not exist resources\admin\NUL`) || strings.Contains(windowsText, `if exist resources\admin xcopy`) {
		t.Fatal("Windows release build permits missing or optional admin assets")
	}
}

func TestDebianPackageUsesKessokuIdentityAndUnprivilegedService(t *testing.T) {
	files := []string{
		"debian/control.tpl",
		"debian/kessoku-api.install",
		"debian/kessoku-api.postinst",
		"debian/kessoku-api.postrm",
		"debian/kessoku-api.prerm",
		"systemd/kessoku-api.service",
	}
	combined := strings.Builder{}
	for _, path := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		combined.Write(contents)
	}
	text := combined.String()
	for _, forbidden := range []string{"Source: rustdesk-api-server", "Package: rustdesk-api-server", "rustdesk-api.service", "bin/rustdesk-api ", "User=\n", "Group=\n"} {
		if strings.Contains(text, forbidden) {
			t.Fatalf("Debian packaging retains unsafe or upstream identity %q", forbidden)
		}
	}
	for _, required := range []string{"Package: kessoku-api", "bin/kessoku-api usr/bin", "User=kessoku-api", "Group=kessoku-api", "ProtectSystem=strict"} {
		if !strings.Contains(text, required) {
			t.Fatalf("Debian packaging is missing %q", required)
		}
	}
}

func TestCandidateDebianBuilderRejectsBrowserClientsAndNormalizesMetadata(t *testing.T) {
	contents, err := os.ReadFile("scripts/build-deb.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	for _, required := range []string{
		"Package: kessoku-api",
		"--root-owner-group -Zgzip",
		"SOURCE_DATE_EPOCH=0",
		"touch -h -d '@0'",
		"-name web -o -name web2",
		"kessoku-api.service",
		"usr/share/doc/kessoku-api/copyright",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("candidate Debian builder is missing %q", required)
		}
	}
}
