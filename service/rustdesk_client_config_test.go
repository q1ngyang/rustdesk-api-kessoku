package service

import (
	"encoding/base64"
	"encoding/json"
	"testing"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
)

func TestRustDeskClientConfigurationUsesOfficialEncoding(t *testing.T) {
	cfg := &config.Config{
		Rustdesk: config.Rustdesk{
			IdServer: "id.example.test:21116", RelayServer: "relay-b.example.test:21117",
			ApiServer: "https://api.example.test", Key: " public-key\n",
		},
		WebClient: config.WebClient{RelayWSSURLs: map[string]string{
			"relay-a.example.test:21117": "wss://relay-a.example.test/ws/relay",
			"relay-b.example.test:21117": "wss://relay-b.example.test/ws/relay",
		}},
	}
	result := RustDeskClientConfigurationFor(cfg)
	if result.Key != "public-key" || result.RelayServer != "" {
		t.Fatalf("configuration was not normalized: %+v", result)
	}

	reversed := []byte(result.ConfigString)
	for left, right := 0, len(reversed)-1; left < right; left, right = left+1, right-1 {
		reversed[left], reversed[right] = reversed[right], reversed[left]
	}
	payload, err := base64.URLEncoding.DecodeString(string(reversed))
	if err != nil {
		t.Fatal(err)
	}
	decoded := map[string]string{}
	if err := json.Unmarshal(payload, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded["host"] != result.IDServer || decoded["relay"] != "" || decoded["api"] != result.APIServer || decoded["key"] != result.Key || len(decoded) != 4 {
		t.Fatalf("official client config payload = %#v", decoded)
	}
}
