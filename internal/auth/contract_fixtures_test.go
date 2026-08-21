package auth

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestInactiveIntrospectionFixturesDoNotExposeCauses(t *testing.T) {
	fixtureNames := []string{
		"introspection-expired.json",
		"introspection-revoked.json",
		"introspection-disabled.json",
		"introspection-rotated-key.json",
	}
	want := map[string]interface{}{"active": false, "reason": "inactive"}
	for _, name := range fixtureNames {
		encoded, err := os.ReadFile(filepath.Join("testdata", name))
		if err != nil {
			t.Fatal(err)
		}
		got := map[string]interface{}{}
		if err := json.Unmarshal(encoded, &got); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("%s leaks an inactive cause or claim: %#v", name, got)
		}
	}
}

func TestJWKSFixtureContainsPublicEd25519KeysOnly(t *testing.T) {
	encoded, err := os.ReadFile(filepath.Join("testdata", "jwks-current-previous.json"))
	if err != nil {
		t.Fatal(err)
	}
	fixture := JWKS{}
	if err := json.Unmarshal(encoded, &fixture); err != nil {
		t.Fatal(err)
	}
	if len(fixture.Keys) != 2 {
		t.Fatalf("fixture key count = %d", len(fixture.Keys))
	}
	for _, key := range fixture.Keys {
		if key.KeyType != "OKP" || key.Curve != "Ed25519" || key.Alg != "EdDSA" || key.Use != "sig" || key.KeyID == "" || key.X == "" {
			t.Fatalf("invalid public JWK fixture: %+v", key)
		}
	}
	if string(encoded) == "" || !json.Valid(encoded) {
		t.Fatal("invalid JWKS fixture")
	}
}
