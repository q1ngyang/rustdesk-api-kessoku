package service

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

func TestStoreImageAcceptsVerifiedRasterAndUsesPrivateRandomPath(t *testing.T) {
	oldConfig := Config
	t.Cleanup(func() { Config = oldConfig })
	root := t.TempDir()
	Config = &config.Config{Media: config.Media{Directory: root, MaxImageBytes: 1 << 20}}

	content := &bytes.Buffer{}
	pixel := image.NewRGBA(image.Rect(0, 0, 1, 1))
	pixel.Set(0, 0, color.RGBA{R: 0x72, G: 0x7f, B: 0xee, A: 0xff})
	if err := png.Encode(content, pixel); err != nil {
		t.Fatal(err)
	}
	mediaURL, err := StoreImage(bytes.NewReader(content.Bytes()), "avatars")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(mediaURL, "/media/avatars/") || !strings.HasSuffix(mediaURL, ".png") || strings.Contains(mediaURL, "..") {
		t.Fatalf("unsafe media URL %q", mediaURL)
	}
	stored := filepath.Join(root, strings.TrimPrefix(mediaURL, "/media/"))
	info, err := os.Stat(stored)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("stored image mode = %o, want 600", info.Mode().Perm())
	}
	if saved, err := os.ReadFile(stored); err != nil || !bytes.Equal(saved, content.Bytes()) {
		t.Fatalf("stored image mismatch: err=%v", err)
	}
}

func TestStoreImageRejectsUntrustedContentAndArea(t *testing.T) {
	oldConfig := Config
	t.Cleanup(func() { Config = oldConfig })
	Config = &config.Config{Media: config.Media{Directory: t.TempDir(), MaxImageBytes: 64}}

	if _, err := StoreImage(strings.NewReader(`<svg xmlns="http://www.w3.org/2000/svg"><script/></svg>`), "branding"); err == nil {
		t.Fatal("accepted active SVG content")
	}
	if _, err := StoreImage(bytes.NewReader(make([]byte, 65)), "branding"); err == nil {
		t.Fatal("accepted content larger than the configured limit")
	}
	if _, err := StoreImage(strings.NewReader("image"), "../escape"); err == nil {
		t.Fatal("accepted a caller-controlled media area")
	}
}

func TestBrandingValidationRestrictsAssetsAndCustomCSS(t *testing.T) {
	valid := &model.BrandingSetting{
		AdminTitle:       "Example Remote",
		AdminLogoURL:     "/media/branding/0123456789abcdef.png",
		LoginCustomHTML:  "<p>Authorized support only.</p>",
		LoginCustomCSS:   ".brand-custom strong { color: #727fee; }",
		WebClientLogoURL: "/media/branding/abcdef0123456789.webp",
		AdminIconURL:     "https://cdn.example.test/remote/icon.png?v=2",
	}
	if err := validateBranding(valid); err != nil {
		t.Fatalf("valid branding rejected: %v", err)
	}

	for name, mutate := range map[string]func(*model.BrandingSetting){
		"insecure external asset": func(value *model.BrandingSetting) { value.AdminLogoURL = "http://cdn.example/logo.png" },
		"path traversal":          func(value *model.BrandingSetting) { value.AdminLogoURL = "/media/../secrets" },
		"CSS URL": func(value *model.BrandingSetting) {
			value.LoginCustomCSS = ".x{background:url(https://evil.example/x)}"
		},
		"fixed overlay": func(value *model.BrandingSetting) { value.LoginCustomCSS = ".x{position: fixed}" },
	} {
		t.Run(name, func(t *testing.T) {
			candidate := *valid
			mutate(&candidate)
			if err := validateBranding(&candidate); err == nil {
				t.Fatal("unsafe branding accepted")
			}
		})
	}
}

func TestSystemSettingValidationRequiresAllThreePublicMMDBSources(t *testing.T) {
	valid := &model.SystemSetting{
		GeoIPEnabled: true, GeoIPCityURL: DefaultGeoIPCityURL,
		GeoIPCountryURL: DefaultGeoIPCountryURL, GeoIPASNURL: DefaultGeoIPASNURL,
		GeoIPUpdateHours: 168,
	}
	if err := validateSystemSetting(valid); err != nil {
		t.Fatalf("valid GeoIP settings rejected: %v", err)
	}
	invalid := *valid
	invalid.GeoIPCountryURL = "http://example.test/GeoLite2-Country.mmdb"
	if err := validateSystemSetting(&invalid); err == nil {
		t.Fatal("accepted insecure Country MMDB URL")
	}
}
