package service

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
	"gorm.io/gorm"
)

const brandingSingletonID uint = 1

type BrandingService struct{}

type PublicBranding struct {
	AdminTitle                  string `json:"admin_title"`
	AdminSubtitle               string `json:"admin_subtitle"`
	BrandLogoLightURL           string `json:"brand_logo_light_url"`
	BrandLogoDarkURL            string `json:"brand_logo_dark_url"`
	BrandIconLightURL           string `json:"brand_icon_light_url"`
	BrandIconDarkURL            string `json:"brand_icon_dark_url"`
	LoginBackgroundLightURL     string `json:"login_background_light_url"`
	LoginBackgroundDarkURL      string `json:"login_background_dark_url"`
	WebClientBackgroundLightURL string `json:"web_client_background_light_url"`
	WebClientBackgroundDarkURL  string `json:"web_client_background_dark_url"`
	LoginKicker                 string `json:"login_kicker"`
	LoginHeading                string `json:"login_heading"`
	LoginCopy                   string `json:"login_copy"`
	FooterHTML                  string `json:"footer_html"`
	LoginCustomHTML             string `json:"login_custom_html"`
	LoginCustomCSS              string `json:"login_custom_css"`
	WebClientTitle              string `json:"web_client_title"`
}

type PublicWebClientBranding struct {
	Title              string `json:"title"`
	LogoLightURL       string `json:"logo_light_url"`
	LogoDarkURL        string `json:"logo_dark_url"`
	IconLightURL       string `json:"icon_light_url"`
	IconDarkURL        string `json:"icon_dark_url"`
	BackgroundLightURL string `json:"background_light_url"`
	BackgroundDarkURL  string `json:"background_dark_url"`
	FooterHTML         string `json:"footer_html"`
}

func themedAsset(light, dark, legacy string) (string, string) {
	if light == "" {
		light = legacy
	}
	if dark == "" {
		dark = legacy
	}
	return light, dark
}

func firstBrandValue(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func (s *BrandingService) Get() (*model.BrandingSetting, error) {
	setting := &model.BrandingSetting{IdModel: model.IdModel{Id: brandingSingletonID}}
	if DB == nil {
		return setting, errors.New("database is unavailable")
	}
	err := DB.First(setting, brandingSingletonID).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return setting, nil
	}
	return setting, err
}

func (s *BrandingService) Public() PublicBranding {
	result := PublicBranding{AdminTitle: Config.Admin.Title}
	if DB == nil {
		return result
	}
	setting, err := s.Get()
	if err != nil {
		Logger.WithError(err).Warn("load public branding")
		return result
	}
	legacyLogoLight := firstBrandValue(setting.AdminLogoLightURL, setting.LoginLogoLightURL, setting.WebClientLogoLightURL, setting.AdminLogoURL, setting.LoginLogoURL, setting.WebClientLogoURL)
	legacyLogoDark := firstBrandValue(setting.AdminLogoDarkURL, setting.LoginLogoDarkURL, setting.WebClientLogoDarkURL, setting.AdminLogoURL, setting.LoginLogoURL, setting.WebClientLogoURL)
	legacyIconLight := firstBrandValue(setting.AdminIconLightURL, setting.WebClientIconLightURL, setting.AdminIconURL, setting.WebClientIconURL)
	legacyIconDark := firstBrandValue(setting.AdminIconDarkURL, setting.WebClientIconDarkURL, setting.AdminIconURL, setting.WebClientIconURL)
	result = PublicBranding{
		AdminTitle: setting.AdminTitle, AdminSubtitle: setting.AdminSubtitle,
		BrandLogoLightURL: firstBrandValue(setting.BrandLogoLightURL, legacyLogoLight), BrandLogoDarkURL: firstBrandValue(setting.BrandLogoDarkURL, legacyLogoDark),
		BrandIconLightURL: firstBrandValue(setting.BrandIconLightURL, legacyIconLight), BrandIconDarkURL: firstBrandValue(setting.BrandIconDarkURL, legacyIconDark),
		LoginBackgroundLightURL:     firstBrandValue(setting.LoginBackgroundLightURL, setting.LoginBackgroundURL),
		LoginBackgroundDarkURL:      firstBrandValue(setting.LoginBackgroundDarkURL, setting.LoginBackgroundURL),
		WebClientBackgroundLightURL: setting.WebClientBackgroundLightURL, WebClientBackgroundDarkURL: setting.WebClientBackgroundDarkURL,
		LoginKicker: setting.LoginKicker, LoginHeading: setting.LoginHeading, LoginCopy: setting.LoginCopy,
		FooterHTML: firstBrandValue(setting.FooterHTML, setting.LoginFooter), LoginCustomHTML: setting.LoginCustomHTML, LoginCustomCSS: setting.LoginCustomCSS,
		WebClientTitle: setting.WebClientTitle,
	}
	if result.AdminTitle == "" {
		result.AdminTitle = Config.Admin.Title
	}
	return result
}

// PublicForWebClient resolves persisted relative media paths against the
// validated API origin because the browser client is intentionally hosted on
// a different HTTPS origin.
func (s *BrandingService) PublicForWebClient() PublicWebClientBranding {
	result := s.Public()
	web := PublicWebClientBranding{
		Title: result.WebClientTitle, LogoLightURL: result.BrandLogoLightURL, LogoDarkURL: result.BrandLogoDarkURL,
		IconLightURL: result.BrandIconLightURL, IconDarkURL: result.BrandIconDarkURL,
		BackgroundLightURL: result.WebClientBackgroundLightURL, BackgroundDarkURL: result.WebClientBackgroundDarkURL,
		FooterHTML: result.FooterHTML,
	}
	for _, target := range []*string{&web.LogoLightURL, &web.LogoDarkURL, &web.IconLightURL, &web.IconDarkURL, &web.BackgroundLightURL, &web.BackgroundDarkURL} {
		if strings.HasPrefix(*target, "/media/") {
			*target = strings.TrimRight(Config.WebClient.APIOrigin, "/") + *target
		}
	}
	return web
}

func (s *BrandingService) UpdateContext(ctx context.Context, actorUserID uint, requestID string, next *model.BrandingSetting) (operationErr error) {
	if next == nil {
		return errors.New("branding payload is required")
	}
	if err := validateBranding(next); err != nil {
		return err
	}
	event, err := beginSecurityAudit(ctx, actorUserID, requestID, "branding.updated", "branding", "1", nil)
	if err != nil {
		return err
	}
	defer finalizeSecurityAudit(event, &operationErr, "branding_update_failed")
	next.Id = brandingSingletonID
	next.UpdatedBy = actorUserID
	return DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		values := map[string]interface{}{
			"admin_title": next.AdminTitle, "admin_subtitle": next.AdminSubtitle,
			"brand_logo_light_url": next.BrandLogoLightURL, "brand_logo_dark_url": next.BrandLogoDarkURL,
			"brand_icon_light_url": next.BrandIconLightURL, "brand_icon_dark_url": next.BrandIconDarkURL,
			"login_background_light_url": next.LoginBackgroundLightURL, "login_background_dark_url": next.LoginBackgroundDarkURL,
			"web_client_background_light_url": next.WebClientBackgroundLightURL, "web_client_background_dark_url": next.WebClientBackgroundDarkURL,
			"login_kicker": next.LoginKicker, "login_heading": next.LoginHeading, "login_copy": next.LoginCopy,
			"footer_html": next.FooterHTML, "login_custom_html": next.LoginCustomHTML, "login_custom_css": next.LoginCustomCSS,
			"web_client_title": next.WebClientTitle,
			// Mirror canonical assets into v307 fields so rollback remains usable.
			"admin_logo_light_url": next.BrandLogoLightURL, "admin_logo_dark_url": next.BrandLogoDarkURL,
			"admin_icon_light_url": next.BrandIconLightURL, "admin_icon_dark_url": next.BrandIconDarkURL,
			"login_logo_light_url": next.BrandLogoLightURL, "login_logo_dark_url": next.BrandLogoDarkURL,
			"web_client_logo_light_url": next.BrandLogoLightURL, "web_client_logo_dark_url": next.BrandLogoDarkURL,
			"web_client_icon_light_url": next.BrandIconLightURL, "web_client_icon_dark_url": next.BrandIconDarkURL,
			"login_background_url": next.LoginBackgroundLightURL, "login_footer": next.FooterHTML,
			"admin_logo_url": "", "admin_icon_url": "", "login_logo_url": "", "web_client_logo_url": "", "web_client_icon_url": "",
			"updated_by": actorUserID,
		}
		current := &model.BrandingSetting{IdModel: model.IdModel{Id: brandingSingletonID}}
		if err := tx.FirstOrCreate(current, model.BrandingSetting{IdModel: model.IdModel{Id: brandingSingletonID}}).Error; err != nil {
			return err
		}
		return tx.Model(current).Updates(values).Error
	})
}

func validateBranding(setting *model.BrandingSetting) error {
	fields := []struct {
		name, value string
		max         int
	}{
		{"admin_title", setting.AdminTitle, 120}, {"admin_subtitle", setting.AdminSubtitle, 120}, {"login_kicker", setting.LoginKicker, 160},
		{"login_heading", setting.LoginHeading, 240}, {"login_copy", setting.LoginCopy, 2000},
		{"footer_html", setting.FooterHTML, 2048}, {"web_client_title", setting.WebClientTitle, 120},
		{"login_custom_html", setting.LoginCustomHTML, 16 << 10}, {"login_custom_css", setting.LoginCustomCSS, 16 << 10},
	}
	for _, field := range fields {
		if len(field.value) > field.max || strings.ContainsRune(field.value, '\x00') {
			return fmt.Errorf("%s is invalid or too long", field.name)
		}
	}
	for name, value := range map[string]string{
		"brand_logo_light_url": setting.BrandLogoLightURL, "brand_logo_dark_url": setting.BrandLogoDarkURL,
		"brand_icon_light_url": setting.BrandIconLightURL, "brand_icon_dark_url": setting.BrandIconDarkURL,
		"login_background_light_url": setting.LoginBackgroundLightURL, "login_background_dark_url": setting.LoginBackgroundDarkURL,
		"web_client_background_light_url": setting.WebClientBackgroundLightURL, "web_client_background_dark_url": setting.WebClientBackgroundDarkURL,
		"admin_logo_url": setting.AdminLogoURL, "admin_icon_url": setting.AdminIconURL,
		"admin_logo_light_url": setting.AdminLogoLightURL, "admin_logo_dark_url": setting.AdminLogoDarkURL,
		"admin_icon_light_url": setting.AdminIconLightURL, "admin_icon_dark_url": setting.AdminIconDarkURL,
		"login_logo_light_url": setting.LoginLogoLightURL, "login_logo_dark_url": setting.LoginLogoDarkURL,
		"login_logo_url": setting.LoginLogoURL, "login_background_url": setting.LoginBackgroundURL,
		"web_client_logo_light_url": setting.WebClientLogoLightURL, "web_client_logo_dark_url": setting.WebClientLogoDarkURL,
		"web_client_icon_light_url": setting.WebClientIconLightURL, "web_client_icon_dark_url": setting.WebClientIconDarkURL,
		"web_client_logo_url": setting.WebClientLogoURL, "web_client_icon_url": setting.WebClientIconURL,
	} {
		if err := validateBrandAssetURL(value); err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
	}
	css := strings.ToLower(setting.LoginCustomCSS)
	css = regexp.MustCompile(`(?s)/\*.*?\*/`).ReplaceAllString(css, "")
	css = strings.NewReplacer(" ", "", "\t", "", "\r", "", "\n", "").Replace(css)
	for _, forbidden := range []string{"@import", "url(", "expression(", "javascript:", "behavior:", "-moz-binding", "position:fixed", "position: fixed", "position:sticky", "position: sticky", "z-index", "</style", "<script"} {
		if strings.Contains(css, forbidden) {
			return fmt.Errorf("login_custom_css contains forbidden construct %q", forbidden)
		}
	}
	return nil
}

func validateBrandAssetURL(value string) error {
	if value == "" {
		return nil
	}
	if len(value) > 512 || strings.TrimSpace(value) != value {
		return errors.New("must be an uploaded /media/ path or an HTTPS image URL")
	}
	parsed, err := url.Parse(value)
	if err != nil || parsed.Fragment != "" || parsed.User != nil {
		return errors.New("contains an invalid image URL")
	}
	if parsed.IsAbs() {
		if parsed.Scheme != "https" || parsed.Host == "" {
			return errors.New("external images must use HTTPS")
		}
		return nil
	}
	if !strings.HasPrefix(value, "/media/") || parsed.RawQuery != "" || strings.Contains(value, "..") {
		return errors.New("contains an invalid media path")
	}
	return nil
}
