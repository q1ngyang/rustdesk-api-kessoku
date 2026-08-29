package main

import (
	"bufio"
	"os"
	"regexp"
	"strings"
	"testing"
)

var immutableAction = regexp.MustCompile(`^\s*(?:-\s*)?uses:\s+[^@\s]+@[0-9a-f]{40}(?:\s+#.*)?$`)

func TestWorkflowActionsUseImmutableCommits(t *testing.T) {
	for _, path := range []string{".github/workflows/ci.yml", ".github/workflows/build.yml", ".github/workflows/release.yml"} {
		file, err := os.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(file)
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := scanner.Text()
			if strings.Contains(line, "uses:") && !immutableAction.MatchString(line) {
				_ = file.Close()
				t.Fatalf("%s:%d action is not pinned to a full commit: %s", path, lineNumber, line)
			}
		}
		if err := scanner.Err(); err != nil {
			_ = file.Close()
			t.Fatal(err)
		}
		if err := file.Close(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCandidateWorkflowCannotPublishOrDownloadUnverifiedToolchains(t *testing.T) {
	contents, err := os.ReadFile(".github/workflows/build.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(contents)
	for _, forbidden := range []string{
		"push: true",
		"packages: write",
		"contents: write",
		"action-gh-release",
		"docker/login-action",
		"changelogithub",
		"musl.ljw.red",
		"wget ",
	} {
		if strings.Contains(workflow, forbidden) {
			t.Fatalf("candidate workflow contains publishing or unverified input %q", forbidden)
		}
	}
	if !strings.Contains(workflow, "permissions:\n  contents: read") {
		t.Fatal("candidate workflow is not explicitly read-only")
	}
	if strings.Contains(workflow, "\n  push:\n") {
		t.Fatal("candidate workflow must require an explicit non-publishing dispatch")
	}
}

func TestSecurityToolDownloadsHaveFixedVersionsAndChecksums(t *testing.T) {
	contents, err := os.ReadFile("scripts/install-ci-tools.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	for _, required := range []string{
		"actionlint_version=1.7.12",
		"actionlint_sha256=8aca8db96f1b94770f1b0d72b6dddcb1ebb8123cb3712530b08cc387b349a3d8",
		"gitleaks_version=8.25.1",
		"gitleaks_sha256=3000d057342489827ee127310771873000b658f2987be7bbd21968ab7443913a",
		"syft_version=1.50.0",
		"syft_sha256=bf7b29ff57f06da30918266a0e1c2885a8f99784798d1bdb1628886aa015d788",
		"sha256sum --check --status",
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("CI tool installer is missing pinned evidence %q", required)
		}
	}
}

func TestWorkflowsRunPinnedActionlint(t *testing.T) {
	for _, path := range []string{".github/workflows/ci.yml", ".github/workflows/build.yml"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(contents), `kessoku-tools/actionlint`) {
			t.Fatalf("%s does not run the checksum-pinned workflow linter", path)
		}
	}
}

func TestPublicationConsumesExactApprovedCandidateAndAttestsIt(t *testing.T) {
	legacyModulePath := "github.com/q1ngyang/rustdesk-api-kessoku/" + "v2"
	contents, err := os.ReadFile(".github/workflows/release.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(contents)
	for _, required := range []string{
		`test "$status" = APPROVED`,
		`test "$release_tag" = "${GITHUB_REF_NAME}"`,
		`test "${GITHUB_REF_TYPE}" = tag`,
		`kessoku-release-candidate-${{ github.sha }}`,
		`actions/attest-build-provenance@96278af6caaf10aea03fd8d33a09a777ca52d62f`,
		`actions/attest-sbom@4651f806c01d8637787e274ac3bdf724ef169f34`,
		`subject-checksums: candidate/release-assets/SHA256SUMS`,
		`artifact-metadata: write`,
		`context: candidate/docker`,
		`driver-opts: image=moby/buildkit@sha256:28a898719c18a33f4e8000685287fa36fd0dd9560c6440227d3a732d79bb41d8`,
		`provenance: mode=max`,
		`sbom: true`,
		`candidate/release-assets/GO-BUILD-INFO.txt`,
		`github.com/q1ngyang/rustdesk-api-kessoku/v3/cmd`,
		`chmod 0755 candidate/docker/release/kessoku-api`,
		`cmp "$archive_binary" candidate/docker/release/kessoku-api`,
		`vcs.revision='"${GITHUB_SHA}"`,
		`vcs.modified=false`,
		`test "$release_tag" = v3.0.4`,
		`test "$(wc -l < release-notes.md)" -le 12`,
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("publication workflow is missing fail-closed control %q", required)
		}
	}
	if !strings.Contains(workflow, "environment: kessoku-release") {
		t.Fatal("publication workflow does not require the release environment")
	}
	if strings.Contains(workflow, legacyModulePath) {
		t.Fatal("publication workflow still validates a project-owned /v2 module path")
	}
	buildContents, err := os.ReadFile(".github/workflows/build.yml")
	if err != nil {
		t.Fatal(err)
	}
	buildWorkflow := string(buildContents)
	if !strings.Contains(buildWorkflow, "github.com/q1ngyang/rustdesk-api-kessoku/v3/cmd") ||
		strings.Contains(buildWorkflow, legacyModulePath) {
		t.Fatal("candidate workflow must validate only the project-owned /v3 build path")
	}
	status, err := os.ReadFile("RELEASE_STATUS")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(status), "status: APPROVED") || !strings.Contains(string(status), "release_tag: v3.0.4") {
		t.Fatal("release source must name the explicitly approved immutable tag")
	}
}

func TestCompilerFrontendAndVulnerabilityScannerArePinned(t *testing.T) {
	files := map[string][]string{
		"go.mod": {
			"go 1.26",
			"toolchain go1.26.6",
		},
		"Dockerfile.dev": {
			"golang:1.26.6-bookworm@sha256:116d58cbd88c1297624acc6e967a060012422bacf9930927e23fb719189c6f36",
			"node:24.15.0-bookworm@sha256:f22d6a1f082c02f292e86929b5b0442ac2e5eaf438a5dea9b1566601c3e05940",
			`test "$(npm --version)" = "11.12.1"`,
		},
		".github/workflows/ci.yml": {
			"govulncheck@v1.7.0",
			"go1.26.6",
		},
		".github/workflows/build.yml": {
			"GO_VERSION: 1.26.6",
			"NODE_VERSION: 24.15.0",
			"NPM_VERSION: 11.12.1",
			"shell: bash",
			`test "$GITHUB_WORKSPACE" = "$PWD"`,
			`git config --global --add safe.directory "$GITHUB_WORKSPACE"`,
			`printf '%s\n' "$mod_before" "$sum_before" | sha256sum --check`,
			`env -u KESSOKU_TEST_POSTGRES_DSN -u KESSOKU_TEST_MYSQL_DSN`,
			"chmod 0755 runtime/release/kessoku-api",
			`grep -Eq '^-rwxr-xr-x '`,
			"npm sbom --omit=dev --sbom-format cyclonedx",
			"npm audit signatures",
			`diff -u "${RUNNER_TEMP}/admin-web-dist-1.sha256"`,
			"cache-dependency-path: |",
			"web-client/package-lock.json",
			`diff -u "${RUNNER_TEMP}/web-client-dist-1.sha256"`,
			"kessoku-web-client.cdx.json",
		},
	}
	for path, requiredValues := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		for _, required := range requiredValues {
			if !strings.Contains(string(contents), required) {
				t.Fatalf("%s is missing pinned input %q", path, required)
			}
		}
	}
}

func TestAdminWebIsEmbeddedWithReviewedProvenance(t *testing.T) {
	const importCommit = "2a9d037fc271cf96b39fd4add4b97c4ff4477f12"
	const seedCommit = "3998c2a9213fcd047252776d0f0db33e6717026c"
	for _, path := range []string{"docs/development/ADMIN-WEB-PROVENANCE.md", "admin-web/PROVENANCE.md"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		for _, commit := range []string{importCommit, seedCommit} {
			if !strings.Contains(text, commit) {
				t.Fatalf("%s does not record admin-web lineage %s", path, commit)
			}
		}
	}
	for _, path := range []string{".github/workflows/build.yml", "Dockerfile.dev"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		if !strings.Contains(text, "admin-web/") {
			t.Fatalf("%s does not build the embedded admin-web source", path)
		}
		for _, forbidden := range []string{"rustdesk-api-kessoku-web", "git fetch", "lejianwen/rustdesk-api-web@master"} {
			if strings.Contains(text, forbidden) {
				t.Fatalf("%s accepts external or moving admin-web input %q", path, forbidden)
			}
		}
	}
}

func TestLocalCandidateVerifierCannotPublishOrSubstituteFrontendSource(t *testing.T) {
	contents, err := os.ReadFile("scripts/verify-local-admin-candidate.sh")
	if err != nil {
		t.Fatal(err)
	}
	script := string(contents)
	for _, required := range []string{
		`admin_web_root="$repo_root/admin-web"`,
		`web_client_root="$repo_root/web-client"`,
		"admin_import_commit=2a9d037fc271cf96b39fd4add4b97c4ff4477f12",
		"admin_seed_commit=3998c2a9213fcd047252776d0f0db33e6717026c",
		"npm ci",
		"npm audit signatures",
		"vcs.modified=false",
		"frame-ancestors 'none'",
		"WEB-CLIENT-DIST-SHA256SUMS",
		"kessoku-web-client.cdx.json",
		`web_client_source/LICENSE`,
		`web_client_source/NOTICE.md`,
		`third-party-licenses/@bufbuild-protobuf-2.9.0.txt`,
		`config.docker-builtin.yaml`,
	} {
		if !strings.Contains(script, required) {
			t.Fatalf("local candidate verifier is missing %q", required)
		}
	}
	for _, forbidden := range []string{"git push", "docker push", "gh release", "npm publish"} {
		if strings.Contains(script, forbidden) {
			t.Fatalf("local candidate verifier contains publishing command %q", forbidden)
		}
	}
}

func TestWebClientThirdPartyLicenceTextIsRequiredInEveryArtifact(t *testing.T) {
	const licencePath = "third-party-licenses/@bufbuild-protobuf-2.9.0.txt"
	for _, path := range []string{
		"Dockerfile",
		"Dockerfile.dev",
		"build.sh",
		"build.bat",
		"scripts/build-deb.sh",
		"scripts/copy-runtime-resources.sh",
		"scripts/verify-local-admin-candidate.sh",
		".github/workflows/ci.yml",
		".github/workflows/build.yml",
		".github/workflows/release.yml",
	} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := strings.ReplaceAll(string(contents), `\`, "/")
		if !strings.Contains(text, licencePath) {
			t.Fatalf("%s does not require the Web Client runtime licence text", path)
		}
	}
}

func TestReleaseBuildsCompileTheModulePackageWithVCSEvidence(t *testing.T) {
	files := map[string][]string{
		"build.sh": {
			"-buildvcs=true -o release/kessoku-api ./cmd",
			"GO-BUILD-INFO.txt",
			"vcs.revision=${KESSOKU_SOURCE_COMMIT}",
			"vcs.modified=false",
		},
		"build.bat": {
			"-buildvcs=true -o release/kessoku-api.exe ./cmd",
			"GO-BUILD-INFO.txt",
			"vcs.revision=%KESSOKU_SOURCE_COMMIT%",
			"vcs.modified=false",
		},
		".github/workflows/build.yml": {
			`test -z "$(git status --porcelain --untracked-files=all)"`,
			`-o "$binary_stage/kessoku-api" ./cmd`,
			`-o "$binary_stage/kessoku-api.rebuild" ./cmd`,
			`cmp "$binary_stage/kessoku-api"`,
			"GO-BUILD-INFO.txt",
			"runtime/GO-BUILD-INFO.txt",
			`vcs.revision='"${GITHUB_SHA}"`,
			"vcs.modified=false",
		},
	}
	for path, requiredValues := range files {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		text := string(contents)
		for _, required := range requiredValues {
			if !strings.Contains(text, required) {
				t.Fatalf("%s does not preserve VCS evidence control %q", path, required)
			}
		}
	}

	contents, err := os.ReadFile("Dockerfile.dev")
	if err != nil {
		t.Fatal(err)
	}
	developmentBuild := string(contents)
	for _, required := range []string{
		"development-only source build intentionally receives no .git directory",
		"-buildvcs=false",
		"-o /out/kessoku-api ./cmd",
	} {
		if !strings.Contains(developmentBuild, required) {
			t.Fatalf("Dockerfile.dev is missing explicit non-release build control %q", required)
		}
	}
	if strings.Contains(developmentBuild, "-buildvcs=true") {
		t.Fatal("Dockerfile.dev cannot claim VCS evidence when .git is excluded")
	}
}

func TestEmbeddedFrontendDerivativesCannotDirtyBackendVCSEvidence(t *testing.T) {
	ignoreContents, err := os.ReadFile(".gitignore")
	if err != nil {
		t.Fatal(err)
	}
	ignore := string(ignoreContents)
	for _, required := range []string{"resources/admin", "resources/client", "web-client/node_modules", "web-client/dist"} {
		if !strings.Contains(ignore, required) {
			t.Fatalf("candidate-generated frontend path is not ignored: %s", required)
		}
	}

	workflowContents, err := os.ReadFile(".github/workflows/build.yml")
	if err != nil {
		t.Fatal(err)
	}
	workflow := string(workflowContents)
	if strings.Contains(workflow, "--if-present") {
		t.Fatal("candidate frontend gates must not be optional")
	}
	for _, required := range []string{
		"working-directory: admin-web",
		"mkdir -p ../resources/admin",
		"mkdir -p ../resources/client",
		`test -z "$(git status --porcelain --untracked-files=all)"`,
	} {
		if !strings.Contains(workflow, required) {
			t.Fatalf("candidate frontend/VCS isolation is missing %q", required)
		}
	}

	dockerIgnoreContents, err := os.ReadFile(".dockerignore")
	if err != nil {
		t.Fatal(err)
	}
	dockerIgnore := string(dockerIgnoreContents)
	if !strings.Contains(dockerIgnore, ".git/") {
		t.Fatal("development Docker context must exclude Git metadata")
	}
	for _, required := range []string{"admin-web/node_modules/", "admin-web/dist/", "web-client/node_modules/", "web-client/dist/"} {
		if !strings.Contains(dockerIgnore, required) {
			t.Fatalf("development Docker context does not exclude derivative %s", required)
		}
	}
}

func TestWorkflowContainerImagesUseImmutableDigests(t *testing.T) {
	for _, path := range []string{".github/workflows/ci.yml", ".github/workflows/build.yml", ".github/workflows/release.yml"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(strings.NewReader(string(contents)))
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "image:") && !strings.Contains(line, "@sha256:") {
				t.Fatalf("%s:%d workflow image is not digest-pinned: %s", path, lineNumber, line)
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
	}
}

func TestDockerBaseImagesUseImmutableDigests(t *testing.T) {
	for _, path := range []string{"Dockerfile", "Dockerfile.dev"} {
		contents, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		scanner := bufio.NewScanner(strings.NewReader(string(contents)))
		lineNumber := 0
		for scanner.Scan() {
			lineNumber++
			line := strings.TrimSpace(scanner.Text())
			if strings.HasPrefix(line, "FROM ") && !strings.Contains(line, "@sha256:") {
				t.Fatalf("%s:%d base image is not digest-pinned: %s", path, lineNumber, line)
			}
		}
		if err := scanner.Err(); err != nil {
			t.Fatal(err)
		}
	}
}
