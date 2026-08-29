package service

import (
	"context"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/model"
)

func TestBrandingEditorDistinguishesLegacyDefaultsFromIntentionalEmptyValues(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.BrandingSetting{}); err != nil {
		t.Fatal(err)
	}
	legacy := &model.BrandingSetting{IdModel: model.IdModel{Id: brandingSingletonID}}
	if err := database.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if current := AllService.BrandingService.Public(); current.DefaultsInitialized {
		t.Fatal("legacy empty branding row was marked initialized")
	}

	next := &model.BrandingSetting{LoginCopy: "", LoginCustomHTML: "", FooterHTML: ""}
	if err := AllService.BrandingService.UpdateContext(context.Background(), 1, "0191f6a0-0000-7000-8000-000000000091", next); err != nil {
		t.Fatal(err)
	}
	stored := &model.BrandingSetting{}
	if err := database.First(stored, brandingSingletonID).Error; err != nil {
		t.Fatal(err)
	}
	if stored.SchemaVersion != 1 {
		t.Fatalf("branding schema version = %d, want 1", stored.SchemaVersion)
	}
	current := AllService.BrandingService.Public()
	if !current.DefaultsInitialized || current.LoginCopy != "" || current.LoginCustomHTML != "" || current.FooterHTML != "" {
		t.Fatalf("intentional empty branding values were not preserved: %+v", current)
	}
}

func TestBrandingOverridesOnlyServerInstanceDisplayName(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.BrandingSetting{}); err != nil {
		t.Fatal(err)
	}
	Config.ServerControl.Instances = []config.StarryInstance{{ID: "primary", Name: "Deployment Primary", Enabled: false}}
	AllService.StarryControlService = NewStarryControlService(Config, Logger, Auth)
	next := &model.BrandingSetting{ServerInstanceNamesJSON: `{"primary":"Osaka production"}`}
	if err := AllService.BrandingService.UpdateContext(context.Background(), 1, "0191f6a0-0000-7000-8000-000000000092", next); err != nil {
		t.Fatal(err)
	}
	instances := AllService.StarryControlService.Instances()
	if len(instances) != 1 || instances[0].ID != "primary" || instances[0].Name != "Osaka production" {
		t.Fatalf("instance label was not applied safely: %+v", instances)
	}
	if err := validateBranding(&model.BrandingSetting{ServerInstanceNamesJSON: `{"unknown":"No"}`}); err == nil {
		t.Fatal("unknown server instance label was accepted")
	}
}

func TestBrandingInstanceNamesColumnUpgradesExistingSQLiteRows(t *testing.T) {
	database := securityAuditDatabase(t, true)
	if err := database.AutoMigrate(&model.BrandingSetting{}); err != nil {
		t.Fatal(err)
	}
	legacy := &model.BrandingSetting{IdModel: model.IdModel{Id: brandingSingletonID}, AdminTitle: "Legacy title"}
	if err := database.Create(legacy).Error; err != nil {
		t.Fatal(err)
	}
	if err := database.Exec("ALTER TABLE branding_settings DROP COLUMN server_instance_names_json").Error; err != nil {
		t.Fatal(err)
	}
	if err := database.AutoMigrate(&model.BrandingSetting{}); err != nil {
		t.Fatalf("upgrade existing branding table: %v", err)
	}
	upgraded := &model.BrandingSetting{}
	if err := database.First(upgraded, brandingSingletonID).Error; err != nil {
		t.Fatal(err)
	}
	if upgraded.AdminTitle != "Legacy title" || upgraded.ServerInstanceNamesJSON != "{}" {
		t.Fatalf("legacy branding row was not preserved with an empty instance-name map: %+v", upgraded)
	}
}
