package starrycontrol

import "testing"

func TestRelayEnrollmentConfigurationDigestMatchesFrozenStarryVector(t *testing.T) {
	wss := "wss://relay.example:21119/ws/telemetry"
	port := 22119
	input := RelayEnrollmentPrepareRequest{
		Version: 1, NodeID: "relay-sg", RelayServer: "relay.example:21117", PublicEndpoint: "relay.example:21117",
		RelayPool: "primary", Profile: "native-wss-fastmedia", WSSEndpoint: &wss,
		ActivateAfterHealth: true, MaxSessions: 1000, CapacityBandwidthBPS: 1_000_000_000,
		FastMediaUDPPort: &port, ExpiresInSeconds: 600,
	}
	digest, err := RelayEnrollmentConfigurationDigest(input)
	if err != nil {
		t.Fatal(err)
	}
	const want = "sha256:af602f02c80294fc66222d100f6fa5065e2b7626c9848041cffdc97c32f8821c"
	if digest != want {
		t.Fatalf("Relay enrollment digest = %s, want frozen Starry vector %s", digest, want)
	}
	input.ExpiresInSeconds = 30
	second, err := RelayEnrollmentConfigurationDigest(input)
	if err != nil || second != digest {
		t.Fatalf("claim lifetime changed approved topology digest: %s, %v", second, err)
	}
	input.FastMediaUDPPort = intPointerForEnrollmentTest(22120)
	changed, err := RelayEnrollmentConfigurationDigest(input)
	if err != nil || changed == digest {
		t.Fatalf("approved UDP endpoint did not change digest: %s, %v", changed, err)
	}
}

func intPointerForEnrollmentTest(value int) *int { return &value }
