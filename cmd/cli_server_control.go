package main

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/servercontrolregistry"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/internal/starrycontrol"
	"github.com/q1ngyang/rustdesk-api-kessoku/v3/service"
	"github.com/spf13/cobra"
)

func newServerControlCommand(configPath *string) *cobra.Command {
	command := &cobra.Command{Use: "server-control", Short: "Manage the independent Starry control registry", Args: noCommandArgs}
	command.AddCommand(newServerControlPairCommand(configPath), newServerControlRegistryCommand(configPath))
	return command
}

func newServerControlPairCommand(configPath *string) *cobra.Command {
	command := &cobra.Command{Use: "pair", Short: "Create allowlisted SP1 Control Agent pairings", Args: noCommandArgs}
	command.AddCommand(newServerControlPairCreateCommand(configPath), newServerControlPairRevokeCommand(configPath))
	return command
}

func newServerControlPairCreateCommand(configPath *string) *cobra.Command {
	var managedID, name, agentOriginID, action, targetInstanceID, confirmation string
	jsonOutput := false
	command := &cobra.Command{
		Use:   "create",
		Short: "Create a short-lived, one-time Control Agent SP1 code",
		Args:  noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if managedID == "" || name == "" || agentOriginID == "" || confirmation == "" {
				return failCommand(cmd, jsonOutput, exitUsage, "ARGUMENT_REQUIRED", "--id, --name, --agent-origin and --confirm are required", "", nil)
			}
			control, closeControl, err := loadControlPairingCommandService(cmd, configPath, jsonOutput)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitConfig, "SERVER_CONTROL_UNAVAILABLE", "cannot open server-control registry", "server-control", err)
			}
			defer closeControl()
			result, err := control.CreateControlPairingLocal(cmd.Context(), service.ControlPairingCreateRequest{
				ManagedID: managedID, Name: name, AgentOriginID: agentOriginID, Action: action,
				TargetInstanceID: targetInstanceID, Confirmation: confirmation,
			})
			if err != nil {
				return failCommand(cmd, jsonOutput, exitServerControl, "PAIRING_CREATE_FAILED", "Control Agent pairing code was not created", "server-control.pairing", err)
			}
			return writePairingCode(cmd, jsonOutput, result)
		},
	}
	command.Flags().StringVar(&managedID, "id", "", "managed instance id")
	command.Flags().StringVar(&name, "name", "", "display name")
	command.Flags().StringVar(&agentOriginID, "agent-origin", "", "deployment allowlist id (never an arbitrary URL)")
	command.Flags().StringVar(&action, "action", servercontrolregistry.ActionPair, "pair, adopt, or rotate")
	command.Flags().StringVar(&targetInstanceID, "target-instance-id", "", "existing Starry instance UUID for adopt/rotate binding")
	command.Flags().StringVar(&confirmation, "confirm", "", "exact confirmation: confirm:<action>:<id>:<agent-origin>")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output containing the one-time code")
	return command
}

func newServerControlPairRevokeCommand(configPath *string) *cobra.Command {
	var enrollmentID, confirmation string
	jsonOutput := false
	command := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke an unclaimed Control Agent SP1 code",
		Args:  noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			control, closeControl, err := loadControlCommandService(cmd, configPath, jsonOutput)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitConfig, "SERVER_CONTROL_UNAVAILABLE", "cannot open existing server-control registry", "server-control", err)
			}
			defer closeControl()
			result, err := control.RevokeControlPairingLocal(localControlContext(cmd.Context()), service.ControlPairingRevokeRequest{
				EnrollmentID: enrollmentID, Confirmation: confirmation,
			})
			if err != nil {
				return failCommand(cmd, jsonOutput, exitServerControl, "PAIRING_REVOKE_FAILED", "Control Agent pairing code was not revoked", "server-control.pairing", err)
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&enrollmentID, "enrollment-id", "", "Control Agent pairing enrollment UUID")
	command.Flags().StringVar(&confirmation, "confirm", "", "exact confirmation: confirm:revoke-pairing:<enrollment-id>")
	command.Flags().BoolVar(&jsonOutput, "json", true, "emit stable JSON output")
	_ = command.MarkFlagRequired("enrollment-id")
	_ = command.MarkFlagRequired("confirm")
	return command
}

func newServerControlRegistryCommand(configPath *string) *cobra.Command {
	command := &cobra.Command{Use: "registry", Short: "Inspect, migrate, or explicitly purge the independent registry", Args: noCommandArgs}
	command.AddCommand(
		newServerControlRegistryStatusCommand(configPath),
		newServerControlRegistryAdoptHostCommand(configPath),
		newServerControlRegistryPurgeCommand(configPath),
	)
	return command
}

func newServerControlRegistryPurgeCommand(configPath *string) *cobra.Command {
	var installationID, confirmation string
	serviceStopped := false
	dataLossUnderstood := false
	jsonOutput := false
	command := &cobra.Command{
		Use: "purge", Short: "Permanently delete the independent registry and all managed identities",
		Args: noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !serviceStopped || !dataLossUnderstood || confirmation != "confirm:purge:"+installationID {
				return failCommand(cmd, jsonOutput, exitUsage, "PURGE_CONFIRMATION_REQUIRED", "require --service-stopped, --data-loss-understood, and exact --confirm confirm:purge:<installation-id>", "confirmation", nil)
			}
			cfg, _, err := loadCommandConfig(configPath)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitConfig, "CONFIG_INVALID", "cannot load configuration", "configuration", err)
			}
			if err := servercontrolregistry.Purge(cmd.Context(), cfg.ServerControl.EffectiveRegistryDirectory(), installationID, confirmation, serviceStopped, dataLossUnderstood); err != nil {
				return failCommand(cmd, jsonOutput, exitServerControl, "REGISTRY_PURGE_FAILED", "registry identity was not deleted", "server-control.registry-directory", err)
			}
			result := map[string]interface{}{"purged": true, "installation_id": installationID}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "registry identity %s permanently purged\n", installationID)
			return nil
		},
	}
	command.Flags().StringVar(&installationID, "installation-id", "", "exact registry installation UUID")
	command.Flags().BoolVar(&serviceStopped, "service-stopped", false, "assert that every Kessoku process sharing this registry is stopped")
	command.Flags().BoolVar(&dataLossUnderstood, "data-loss-understood", false, "second confirmation that pairing identities and recovery state will be irrecoverably deleted")
	command.Flags().StringVar(&confirmation, "confirm", "", "exact confirmation: confirm:purge:<installation-id>")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}

func newServerControlRegistryStatusCommand(configPath *string) *cobra.Command {
	jsonOutput := false
	command := &cobra.Command{
		Use: "status", Short: "Preflight registry path, permissions, schema, host binding, and generation",
		Args: noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, _, err := loadCommandConfig(configPath)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitConfig, "CONFIG_INVALID", "cannot load configuration", "configuration", err)
			}
			root, err := filepath.Abs(cfg.ServerControl.EffectiveRegistryDirectory())
			if err != nil {
				return err
			}
			registry, err := servercontrolregistry.OpenExisting(root, servercontrolregistry.OpenOptions{HostIdentityFile: cfg.ServerControl.EffectiveHostIdentityFile()})
			if err != nil {
				if errors.Is(err, servercontrolregistry.ErrNotFound) {
					return failCommand(cmd, jsonOutput, exitServerControl, "REGISTRY_NOT_INITIALIZED", "no existing server-control registry was found; verify the data path or use an exact confirmed new pair to initialize a new identity", "server-control.registry-directory", err)
				}
				return failCommand(cmd, jsonOutput, exitServerControl, "REGISTRY_PREFLIGHT_FAILED", "registry preflight failed; do not start a pairing writer", "server-control.registry-directory", err)
			}
			defer registry.Close()
			metadata, err := registry.Metadata(cmd.Context())
			if err != nil {
				return err
			}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), metadata)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "registry: %s\nschema: %d\ngeneration: %d\ninstallation: %s\nhost binding: %s\n", metadata.Root, metadata.SchemaVersion, metadata.Generation, metadata.InstallationID, metadata.HostFingerprint)
			return nil
		},
	}
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}

func newServerControlRegistryAdoptHostCommand(configPath *string) *cobra.Command {
	var installationID, confirmation string
	oldHostStopped := false
	jsonOutput := false
	command := &cobra.Command{
		Use: "adopt-host", Short: "Explicitly rebind a restored registry after the old host is stopped",
		Args: noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			if !oldHostStopped || confirmation != "confirm:adopt-host:"+installationID {
				return failCommand(cmd, jsonOutput, exitUsage, "MIGRATION_CONFIRMATION_REQUIRED", "require --old-host-stopped and exact --confirm confirm:adopt-host:<installation-id>", "confirmation", nil)
			}
			cfg, _, err := loadCommandConfig(configPath)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitConfig, "CONFIG_INVALID", "cannot load configuration", "configuration", err)
			}
			if err := servercontrolregistry.AdoptHost(cmd.Context(), cfg.ServerControl.EffectiveRegistryDirectory(), installationID, servercontrolregistry.OpenOptions{HostIdentityFile: cfg.ServerControl.EffectiveHostIdentityFile()}); err != nil {
				return failCommand(cmd, jsonOutput, exitServerControl, "REGISTRY_HOST_ADOPTION_FAILED", "registry host binding was not changed", "server-control.registry-directory", err)
			}
			result := map[string]interface{}{"adopted": true, "installation_id": installationID}
			if jsonOutput {
				return writeJSON(cmd.OutOrStdout(), result)
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "registry host binding adopted for installation %s\n", installationID)
			return nil
		},
	}
	command.Flags().StringVar(&installationID, "installation-id", "", "exact restored registry installation UUID")
	command.Flags().BoolVar(&oldHostStopped, "old-host-stopped", false, "assert that the source Kessoku host cannot write this identity")
	command.Flags().StringVar(&confirmation, "confirm", "", "exact confirmation: confirm:adopt-host:<installation-id>")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output")
	return command
}

func newRelayCommand(configPath *string) *cobra.Command {
	command := &cobra.Command{Use: "relay", Short: "Manage Agent-authorized Relay enrollment", Args: noCommandArgs}
	join := &cobra.Command{Use: "join", Short: "Create and manage Relay SP1 enrollment", Args: noCommandArgs}
	join.AddCommand(
		newRelayJoinCreateCommand(configPath), newRelayJoinListCommand(configPath),
		newRelayJoinActivateCommand(configPath), newRelayJoinRevokeCommand(configPath),
	)
	command.AddCommand(join)
	return command
}

func newRelayJoinCreateCommand(configPath *string) *cobra.Command {
	var instanceID, nodeID, relayServer, publicEndpoint, relayPool, profile, wssEndpoint, idempotencyKey, confirmation string
	var maxSessions int
	var capacity uint64
	var fastMediaPort int
	var expires int
	var draining, activateAfterHealth, jsonOutput bool
	command := &cobra.Command{
		Use: "create", Short: "Ask an authenticated Starry Agent to authorize a Relay SP1 code",
		Args: noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			control, closeControl, err := loadControlCommandService(cmd, configPath, jsonOutput)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitConfig, "SERVER_CONTROL_UNAVAILABLE", "cannot open server-control registry", "server-control", err)
			}
			defer closeControl()
			if idempotencyKey == "" {
				idempotencyKey = uuid.NewString()
			}
			request := starrycontrol.RelayEnrollmentPrepareRequest{
				Version: 1, NodeID: nodeID, RelayServer: relayServer, PublicEndpoint: publicEndpoint,
				RelayPool: relayPool, Profile: profile, ActivateAfterHealth: activateAfterHealth,
				MaxSessions: maxSessions, CapacityBandwidthBPS: capacity, Draining: draining,
				ExpiresInSeconds: expires,
			}
			if cmd.Flags().Changed("wss-endpoint") {
				request.WSSEndpoint = &wssEndpoint
			}
			if cmd.Flags().Changed("fast-media-udp-port") {
				request.FastMediaUDPPort = &fastMediaPort
			}
			ctx := localControlContext(cmd.Context())
			result, err := control.CreateRelayPairingLocal(ctx, service.RelayPairingCreateRequest{
				InstanceID: instanceID, Enrollment: request, IdempotencyKey: idempotencyKey, Confirmation: confirmation,
			})
			if err != nil {
				return failCommand(cmd, jsonOutput, exitServerControl, "RELAY_PAIRING_CREATE_FAILED", "Relay pairing code was not created", "relay.enrollment", err)
			}
			return writePairingCode(cmd, jsonOutput, result)
		},
	}
	command.Flags().StringVar(&instanceID, "starry", "", "configured or managed Starry instance id")
	command.Flags().StringVar(&nodeID, "node-id", "", "exact Relay node id")
	command.Flags().StringVar(&relayServer, "relay-server", "", "Agent-approved Relay server endpoint")
	command.Flags().StringVar(&publicEndpoint, "public-endpoint", "", "Agent-approved public endpoint")
	command.Flags().StringVar(&relayPool, "relay-pool", "", "Agent-approved Relay pool")
	command.Flags().StringVar(&profile, "profile", "native", "native, native-wss, or native-wss-fastmedia")
	command.Flags().StringVar(&wssEndpoint, "wss-endpoint", "", "exact WSS telemetry endpoint")
	command.Flags().IntVar(&maxSessions, "max-sessions", 10000, "maximum sessions")
	command.Flags().Uint64Var(&capacity, "capacity-bandwidth-bps", 1_000_000_000, "bandwidth capacity in bits per second")
	command.Flags().BoolVar(&draining, "draining", false, "create the Relay in draining state")
	command.Flags().IntVar(&fastMediaPort, "fast-media-udp-port", 0, "FastMedia UDP port")
	command.Flags().IntVar(&expires, "expires-in-seconds", 600, "Agent pending-enrollment lifetime")
	command.Flags().BoolVar(&activateAfterHealth, "activate-after-health", false, "immutably pre-authorize health-gated activation (high risk)")
	command.Flags().StringVar(&idempotencyKey, "idempotency-key", "", "safe retry key; generated if omitted")
	command.Flags().StringVar(&confirmation, "confirm", "", "for auto activation: confirm:activate-after-health:<starry>:<node-id>")
	command.Flags().BoolVar(&jsonOutput, "json", false, "emit stable JSON output containing the one-time code")
	_ = command.MarkFlagRequired("starry")
	_ = command.MarkFlagRequired("node-id")
	_ = command.MarkFlagRequired("relay-server")
	_ = command.MarkFlagRequired("public-endpoint")
	_ = command.MarkFlagRequired("relay-pool")
	return command
}

func newRelayJoinListCommand(configPath *string) *cobra.Command {
	var instanceID string
	jsonOutput := false
	command := &cobra.Command{
		Use: "list", Short: "List Agent-redacted Relay enrollment summaries", Args: noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			control, closeControl, err := loadControlCommandService(cmd, configPath, jsonOutput)
			if err != nil {
				return err
			}
			defer closeControl()
			result, err := control.ListRelayEnrollmentsLocal(localControlContext(cmd.Context()), instanceID)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitServerControl, "RELAY_ENROLLMENT_LIST_FAILED", "cannot list Relay enrollments", "relay.enrollment", err)
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&instanceID, "starry", "", "Starry instance id")
	command.Flags().BoolVar(&jsonOutput, "json", true, "emit stable JSON output")
	_ = command.MarkFlagRequired("starry")
	return command
}

func newRelayJoinActivateCommand(configPath *string) *cobra.Command {
	var instanceID, enrollmentID, digest, operationID, healthSnapshotID, confirmation string
	var generation uint64
	jsonOutput := false
	command := &cobra.Command{
		Use: "activate", Short: "Submit exact generation and health ACK evidence to Starry", Args: noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			control, closeControl, err := loadControlCommandService(cmd, configPath, jsonOutput)
			if err != nil {
				return err
			}
			defer closeControl()
			result, err := control.ActivateRelayEnrollmentLocal(localControlContext(cmd.Context()), instanceID, starrycontrol.RelayEnrollmentActivateRequest{
				Version: 1, EnrollmentID: enrollmentID, ConfigurationDigest: digest, OperationID: operationID,
				ConfigGeneration: generation, HealthSnapshotID: healthSnapshotID,
			}, confirmation)
			if err != nil {
				return failCommand(cmd, jsonOutput, exitServerControl, "RELAY_ACTIVATION_FAILED", "Agent rejected Relay activation evidence", "relay.enrollment", err)
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&instanceID, "starry", "", "Starry instance id")
	command.Flags().StringVar(&enrollmentID, "enrollment-id", "", "Relay enrollment UUID")
	command.Flags().StringVar(&digest, "configuration-digest", "", "exact Agent-approved sha256 digest")
	command.Flags().StringVar(&operationID, "operation-id", "", "successful configuration operation UUID")
	command.Flags().Uint64Var(&generation, "generation", 0, "activated Starry configuration generation")
	command.Flags().StringVar(&healthSnapshotID, "health-snapshot-id", "", "Agent health snapshot id")
	command.Flags().StringVar(&confirmation, "confirm", "", "exact confirm:relay-activate:<enrollment-id>:<operation-id>:<generation>")
	command.Flags().BoolVar(&jsonOutput, "json", true, "emit stable JSON output")
	for _, flag := range []string{"starry", "enrollment-id", "configuration-digest", "operation-id", "generation", "health-snapshot-id", "confirm"} {
		_ = command.MarkFlagRequired(flag)
	}
	return command
}

func newRelayJoinRevokeCommand(configPath *string) *cobra.Command {
	var instanceID, enrollmentID, digest string
	jsonOutput := false
	command := &cobra.Command{
		Use: "revoke", Short: "Request Agent-authoritative Relay enrollment revocation", Args: noCommandArgsJSON(&jsonOutput),
		RunE: func(cmd *cobra.Command, _ []string) error {
			control, closeControl, err := loadControlCommandService(cmd, configPath, jsonOutput)
			if err != nil {
				return err
			}
			defer closeControl()
			result, err := control.RevokeRelayEnrollmentLocal(localControlContext(cmd.Context()), instanceID, starrycontrol.RelayEnrollmentRevokeRequest{
				Version: 1, EnrollmentID: enrollmentID, ConfigurationDigest: digest,
			})
			if err != nil {
				return failCommand(cmd, jsonOutput, exitServerControl, "RELAY_REVOCATION_FAILED", "Agent rejected Relay enrollment revocation", "relay.enrollment", err)
			}
			return writeJSON(cmd.OutOrStdout(), result)
		},
	}
	command.Flags().StringVar(&instanceID, "starry", "", "Starry instance id")
	command.Flags().StringVar(&enrollmentID, "enrollment-id", "", "Relay enrollment UUID")
	command.Flags().StringVar(&digest, "configuration-digest", "", "exact Agent-approved sha256 digest")
	command.Flags().BoolVar(&jsonOutput, "json", true, "emit stable JSON output")
	for _, flag := range []string{"starry", "enrollment-id", "configuration-digest"} {
		_ = command.MarkFlagRequired(flag)
	}
	return command
}

func loadControlCommandService(cmd *cobra.Command, configPath *string, jsonOutput bool) (*service.StarryControlService, func(), error) {
	return loadControlCommandServiceWithPolicy(cmd, configPath, jsonOutput, false)
}

func loadControlPairingCommandService(cmd *cobra.Command, configPath *string, jsonOutput bool) (*service.StarryControlService, func(), error) {
	return loadControlCommandServiceWithPolicy(cmd, configPath, jsonOutput, true)
}

func loadControlCommandServiceWithPolicy(cmd *cobra.Command, configPath *string, jsonOutput, allowInitialization bool) (*service.StarryControlService, func(), error) {
	cfg, _, err := loadCommandConfig(configPath)
	if err != nil {
		return nil, func() {}, err
	}
	cfg.Rustdesk.LoadKeyFile()
	control := service.NewStarryControlService(cfg, commandLogger(cmd, jsonOutput), nil)
	status := control.PairingStatusLocal()
	initializable := allowInitialization && status.ErrorCode == "PAIRING_REGISTRY_NOT_INITIALIZED"
	if !status.Enabled || (!status.Available && !initializable) {
		_ = control.Close()
		return nil, func() {}, errors.New("SP1 pairing is disabled or its independent registry failed preflight")
	}
	return control, func() { _ = control.Close() }, nil
}

func localControlContext(ctx context.Context) context.Context {
	return starrycontrol.WithRequestMetadata(ctx, starrycontrol.RequestMetadata{RequestID: requestID(), Service: true})
}

func writePairingCode(cmd *cobra.Command, jsonOutput bool, result service.PairingCodeResult) error {
	if jsonOutput {
		return writeJSON(cmd.OutOrStdout(), result)
	}
	_, err := fmt.Fprintf(cmd.OutOrStdout(), "pairing code (displayed once; do not pass it as an argument or environment variable):\n%s\nenrollment: %s\nexpires_at_unix: %d\n", result.Code, result.EnrollmentID, result.ExpiresAtUnix)
	return err
}
