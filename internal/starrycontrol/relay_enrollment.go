package starrycontrol

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

// RelayEnrollmentConfigurationDigest implements Starry Pairing v1's frozen
// canonical Relay approval digest. Expiry is intentionally excluded: it is a
// lifetime for the one-time claim, not part of the approved Relay topology.
func RelayEnrollmentConfigurationDigest(input RelayEnrollmentPrepareRequest) (string, error) {
	// Fields are declared in lexical JSON-key order, matching Starry's
	// recursively sorted canonical JSON representation. Nil optional values are
	// serialized as JSON null and therefore remain part of the binding.
	approved := struct {
		ActivateAfterHealth  bool    `json:"activate_after_health"`
		CapacityBandwidthBPS uint64  `json:"capacity_bandwidth_bps"`
		Draining             bool    `json:"draining"`
		FastMediaUDPPort     *int    `json:"fast_media_udp_port"`
		MaxSessions          int     `json:"max_sessions"`
		NodeID               string  `json:"node_id"`
		Profile              string  `json:"profile"`
		PublicEndpoint       string  `json:"public_endpoint"`
		RelayPool            string  `json:"relay_pool"`
		RelayServer          string  `json:"relay_server"`
		WSSEndpoint          *string `json:"wss_endpoint"`
	}{
		ActivateAfterHealth:  input.ActivateAfterHealth,
		CapacityBandwidthBPS: input.CapacityBandwidthBPS,
		Draining:             input.Draining,
		FastMediaUDPPort:     input.FastMediaUDPPort,
		MaxSessions:          input.MaxSessions,
		NodeID:               input.NodeID,
		Profile:              input.Profile,
		PublicEndpoint:       input.PublicEndpoint,
		RelayPool:            input.RelayPool,
		RelayServer:          input.RelayServer,
		WSSEndpoint:          input.WSSEndpoint,
	}
	raw, err := json.Marshal(approved)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}
