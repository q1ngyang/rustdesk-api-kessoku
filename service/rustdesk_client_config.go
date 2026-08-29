package service

import (
	"encoding/base64"
	"encoding/json"
	"strings"

	"github.com/q1ngyang/rustdesk-api-kessoku/v3/config"
)

// RustDeskClientConfiguration is the deployment information users enter in
// Settings > Network in an official RustDesk client. ConfigString follows the
// official client's ServerConfig encoding so the same value can be copied or
// scanned as a QR code without a Kessoku-specific decoder.
type RustDeskClientConfiguration struct {
	IDServer     string `json:"id_server"`
	RelayServer  string `json:"relay_server"`
	APIServer    string `json:"api_server"`
	Key          string `json:"key"`
	ConfigString string `json:"config_string"`
}

type officialRustDeskServerConfig struct {
	Host  string `json:"host"`
	Relay string `json:"relay"`
	API   string `json:"api"`
	Key   string `json:"key"`
}

func RustDeskClientConfigurationFor(cfg *config.Config) RustDeskClientConfiguration {
	if cfg == nil {
		return RustDeskClientConfiguration{}
	}
	key := strings.TrimSpace(cfg.Rustdesk.Key)
	result := RustDeskClientConfiguration{
		IDServer: strings.TrimSpace(cfg.Rustdesk.IdServer),
		// The portable configuration deliberately leaves Relay blank. The ID
		// server then selects a compatible native/WSS/mixed relay automatically.
		RelayServer: "",
		APIServer:   strings.TrimSpace(cfg.Rustdesk.ApiServer),
		Key:         key,
	}
	// Keep relay empty in the portable string. Official clients will use the
	// rendezvous server's relay choice unless the user explicitly selects the
	// same relay node on both endpoints.
	result.ConfigString = EncodeRustDeskServerConfig(result.IDServer, "", result.APIServer, result.Key)
	return result
}

func EncodeRustDeskServerConfig(host, relay, api, key string) string {
	payload, err := json.Marshal(officialRustDeskServerConfig{Host: host, Relay: relay, API: api, Key: key})
	if err != nil {
		return ""
	}
	encoded := []byte(base64.URLEncoding.EncodeToString(payload))
	for left, right := 0, len(encoded)-1; left < right; left, right = left+1, right-1 {
		encoded[left], encoded[right] = encoded[right], encoded[left]
	}
	return string(encoded)
}
