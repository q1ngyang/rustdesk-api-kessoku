package http

import (
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/global"
)

func TestInternalAuthServerRequiresEd25519Profile(t *testing.T) {
	oldConfig, oldAuth := global.Config, global.Auth
	t.Cleanup(func() { global.Config, global.Auth = oldConfig, oldAuth })
	global.Config.Auth.Internal = config.InternalAuth{Enabled: true}
	global.Auth = nil
	err := StartInternalAuthServer()
	if err == nil || !strings.Contains(err.Error(), "Ed25519") {
		t.Fatalf("startup error = %v", err)
	}
}
