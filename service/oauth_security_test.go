package service

import (
	"strings"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v2/config"
	"github.com/q1ngyang/rustdesk-api-kessoku/v2/model"
)

func TestOauthStateIsBoundedToInitiatingDeviceAndConsumedOnce(t *testing.T) {
	oldConfig, oldCache := Config, OauthCache
	t.Cleanup(func() { Config, OauthCache = oldConfig, oldCache })
	Config = &config.Config{Rustdesk: config.Rustdesk{ApiServer: "https://api.example.test"}}
	OauthCache = &oauthStateStore{items: make(map[string]*OauthCacheItem)}
	service := &OauthService{}
	err, state, _, _, _ := service.BeginAuth(model.OauthTypeWebauth)
	if err != nil {
		t.Fatal(err)
	}
	if len(state) != 32 {
		t.Fatalf("state length = %d", len(state))
	}
	item := &OauthCacheItem{Action: OauthActionTypeLogin, Op: model.OauthTypeWebauth, Id: "device-1", Uuid: "uuid-1"}
	if err := service.SetOauthCache(state, item, 300); err != nil {
		t.Fatal(err)
	}
	if _, matched := service.TakeOauthLoginResult(state, "other-device", "uuid-1"); matched {
		t.Fatal("OAuth state transferred to a different device")
	}
	if pending, matched := service.TakeOauthLoginResult(state, "device-1", "uuid-1"); !matched || pending.UserId != 0 {
		t.Fatal("matching device could not poll pending OAuth state")
	}
	item.UserId = 42
	if err := service.SetOauthCache(state, item, 0); err != nil {
		t.Fatal(err)
	}
	if complete, matched := service.TakeOauthLoginResult(state, "device-1", "uuid-1"); !matched || complete.UserId != 42 {
		t.Fatal("matching device could not consume OAuth result")
	}
	if _, matched := service.TakeOauthLoginResult(state, "device-1", "uuid-1"); matched {
		t.Fatal("OAuth result was consumable more than once")
	}
}

func TestOauthEndpointValidationRejectsLocalDestinations(t *testing.T) {
	for _, endpoint := range []string{"http://issuer.example.test", "https://127.0.0.1", "https://[::1]", "https://metadata.localhost"} {
		if err := validateOauthEndpointURL(endpoint); err == nil {
			t.Fatalf("unsafe endpoint accepted: %s", endpoint)
		}
	}
	if err := validateOauthEndpointURL("https://issuer.example.test/tenant"); err != nil {
		t.Fatal(err)
	}
}

func TestLDAPFilterEscapesUserInputAndIdentityIsImmutable(t *testing.T) {
	filter := (&LdapService{}).filterField("uid", "alice*)(|(uid=*)")
	if strings.Contains(filter, "|(uid=*)") || !strings.Contains(filter, `\2a`) {
		t.Fatalf("unsafe LDAP filter = %q", filter)
	}
	config := &config.Ldap{Url: "ldaps://directory.example.test:636", BaseDn: "dc=example,dc=test"}
	providerA, subjectA, err := ldapIdentityFingerprints(config, "uid=Alice,ou=People,dc=example,dc=test")
	if err != nil {
		t.Fatal(err)
	}
	providerB, subjectB, err := ldapIdentityFingerprints(config, "UID=alice,OU=people,DC=example,DC=test")
	if err != nil {
		t.Fatal(err)
	}
	if providerA != providerB || subjectA != subjectB || len(providerA) != 64 || len(subjectA) != 64 {
		t.Fatal("LDAP identity fingerprints are not stable and bounded")
	}
}
