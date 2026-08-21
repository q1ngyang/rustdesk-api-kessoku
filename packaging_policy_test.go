package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRuntimePackagingIncludesReviewedFrontendsAndExcludesHistoricalClients(t *testing.T) {
	source := filepath.Join(t.TempDir(), "source")
	for _, directory := range []string{"i18n", "public", "templates", "admin", "client"} {
		if err := os.MkdirAll(filepath.Join(source, directory), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{
		"version",
		"admin/index.html",
		"client/index.html",
		"client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt",
	} {
		if err := os.MkdirAll(filepath.Dir(filepath.Join(source, path)), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(source, path), []byte("reviewed\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	destination := filepath.Join(t.TempDir(), "resources")
	command := exec.Command("sh", "scripts/copy-runtime-resources.sh", destination, source, "require-admin", "require-client")
	if output, err := command.CombinedOutput(); err != nil {
		t.Fatalf("copy runtime resources: %v: %s", err, output)
	}
	for _, required := range []string{
		"i18n",
		"public",
		"templates",
		"version",
		"admin",
		"client",
		"client/third-party-licenses/@bufbuild-protobuf-2.9.0.txt",
	} {
		if _, err := os.Stat(filepath.Join(destination, required)); err != nil {
			t.Fatalf("runtime resource %q missing: %v", required, err)
		}
	}
	for _, forbidden := range []string{"web", "web2"} {
		if _, err := os.Stat(filepath.Join(destination, forbidden)); !os.IsNotExist(err) {
			t.Fatalf("historical browser-client directory %q entered runtime package", forbidden)
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
	for _, excluded := range []string{"resources/web/", "resources/web2/", "web-client/node_modules/", "web-client/dist/"} {
		if !strings.Contains(string(dockerIgnore), excluded) {
			t.Fatalf("Docker build context does not exclude %s", excluded)
		}
	}
}

func TestLocalReleaseBuildsRequireBothReviewedFrontends(t *testing.T) {
	linuxBuild, err := os.ReadFile("build.sh")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(linuxBuild), "resources resources require-admin require-client") {
		t.Fatal("Linux release build does not require reviewed frontend assets")
	}
	windowsBuild, err := os.ReadFile("build.bat")
	if err != nil {
		t.Fatal(err)
	}
	windowsText := string(windowsBuild)
	for _, required := range []string{"call :build_frontend admin-web", `call :build_frontend web-client web-client resources\client web-client\LICENSE`, `xcopy resources\admin`, `xcopy resources\client`, `copy web-client\NOTICE.md release\WEB-CLIENT-NOTICE.md`} {
		if !strings.Contains(windowsText, required) {
			t.Fatalf("Windows release build is missing %q", required)
		}
	}
}

func TestComposeUsesMountedBuiltinClientYAML(t *testing.T) {
	compose, err := os.ReadFile("docker-compose.yaml")
	if err != nil {
		t.Fatal(err)
	}
	example, err := os.ReadFile("examples/config.docker-builtin.yaml")
	if err != nil {
		t.Fatal(err)
	}
	for _, required := range []string{
		`${KESSOKU_CONFIG_FILE:-./config.yaml}:/app/conf/config.yaml:ro`,
		`relay-wss-urls is an exact YAML map`,
	} {
		if !strings.Contains(string(compose), required) {
			t.Fatalf("Compose is missing builtin-client configuration control %q", required)
		}
	}
	for _, required := range []string{
		`mode: "builtin"`,
		`listen: "0.0.0.0:21122"`,
		`relay-wss-urls:`,
		`audiences:`,
		`- "kessoku-api"`,
		`- "rustdesk-connect"`,
	} {
		if !strings.Contains(string(example), required) {
			t.Fatalf("builtin Docker configuration is missing %q", required)
		}
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
		"resources/client/index.html",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("candidate Debian builder is missing %q", required)
		}
	}
}
