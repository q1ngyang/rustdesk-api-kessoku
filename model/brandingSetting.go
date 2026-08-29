package model

// BrandingSetting is a singleton deployment-brand record. StarryLinks' logo
// on Server Control is deliberately not represented here: it is a fixed
// product/trust marker and cannot be overridden by tenant branding.
type BrandingSetting struct {
	IdModel
	// SchemaVersion distinguishes legacy all-empty rows from an operator who
	// deliberately cleared optional copy after opening the modern editor.
	SchemaVersion uint   `json:"-" gorm:"not null;default:0"`
	AdminTitle    string `json:"admin_title" gorm:"size:120;not null;default:''"`
	AdminSubtitle string `json:"admin_subtitle" gorm:"size:120;not null;default:''"`
	// BrandLogo* and BrandIcon* are the single deployment identity used by the
	// sign-in page, administration console, About page and WebClient. Keeping
	// one themed set prevents the different surfaces from drifting apart.
	BrandLogoLightURL string `json:"brand_logo_light_url" gorm:"type:text;not null"`
	BrandLogoDarkURL  string `json:"brand_logo_dark_url" gorm:"type:text;not null"`
	BrandIconLightURL string `json:"brand_icon_light_url" gorm:"type:text;not null"`
	BrandIconDarkURL  string `json:"brand_icon_dark_url" gorm:"type:text;not null"`

	LoginBackgroundLightURL     string `json:"login_background_light_url" gorm:"type:text;not null"`
	LoginBackgroundDarkURL      string `json:"login_background_dark_url" gorm:"type:text;not null"`
	WebClientBackgroundLightURL string `json:"web_client_background_light_url" gorm:"type:text;not null"`
	WebClientBackgroundDarkURL  string `json:"web_client_background_dark_url" gorm:"type:text;not null"`
	FooterHTML                  string `json:"footer_html" gorm:"type:text;not null"`

	// The v307 per-surface assets remain as compatibility columns for a
	// reversible v309 migration. New API responses and writes use only the
	// canonical shared fields above.
	AdminLogoLightURL string `json:"admin_logo_light_url" gorm:"type:text;not null"`
	AdminLogoDarkURL  string `json:"admin_logo_dark_url" gorm:"type:text;not null"`
	AdminIconLightURL string `json:"admin_icon_light_url" gorm:"type:text;not null"`
	AdminIconDarkURL  string `json:"admin_icon_dark_url" gorm:"type:text;not null"`
	// Legacy single-theme fields are retained for a reversible migration and
	// older API clients. New writes clear them after copying their value into
	// both themed fields.
	AdminLogoURL       string `json:"admin_logo_url" gorm:"type:text;not null"`
	AdminIconURL       string `json:"admin_icon_url" gorm:"type:text;not null"`
	LoginLogoLightURL  string `json:"login_logo_light_url" gorm:"type:text;not null"`
	LoginLogoDarkURL   string `json:"login_logo_dark_url" gorm:"type:text;not null"`
	LoginLogoURL       string `json:"login_logo_url" gorm:"type:text;not null"`
	LoginBackgroundURL string `json:"login_background_url" gorm:"type:text;not null"`
	LoginKicker        string `json:"login_kicker" gorm:"size:160;not null;default:''"`
	LoginHeading       string `json:"login_heading" gorm:"size:240;not null;default:''"`
	LoginCopy          string `json:"login_copy" gorm:"type:text;not null"`
	LoginFooter        string `json:"login_footer" gorm:"type:text;not null"`
	LoginCustomHTML    string `json:"login_custom_html" gorm:"type:text;not null"`
	LoginCustomCSS     string `json:"login_custom_css" gorm:"type:text;not null"`
	WebClientTitle     string `json:"web_client_title" gorm:"size:120;not null;default:''"`
	// ServerInstanceNamesJSON stores operator-facing labels keyed by the
	// immutable deployment instance ID. Connection details and credentials
	// remain configuration-file owned.
	ServerInstanceNamesJSON string `json:"-" gorm:"type:varchar(8192);not null;default:'{}'"`
	WebClientLogoLightURL   string `json:"web_client_logo_light_url" gorm:"type:text;not null"`
	WebClientLogoDarkURL    string `json:"web_client_logo_dark_url" gorm:"type:text;not null"`
	WebClientIconLightURL   string `json:"web_client_icon_light_url" gorm:"type:text;not null"`
	WebClientIconDarkURL    string `json:"web_client_icon_dark_url" gorm:"type:text;not null"`
	WebClientLogoURL        string `json:"web_client_logo_url" gorm:"type:text;not null"`
	WebClientIconURL        string `json:"web_client_icon_url" gorm:"type:text;not null"`
	Announcement            string `json:"-" gorm:"type:text;not null"` // legacy v304 column; migrated to SystemSetting
	UpdatedBy               uint   `json:"updated_by" gorm:"not null;default:0;index"`
	TimeModel
}
