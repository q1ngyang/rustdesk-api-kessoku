package starrycontrol

import "context"

// ServerControlProvider is the only control-plane seam exposed to Kessoku
// services. It contains domain operations, never raw commands, arbitrary URLs,
// file paths, or transport-client types.
type ServerControlProvider interface {
	Capabilities(context.Context) (Capabilities, error)
	Status(context.Context) (Status, error)
	Relays(context.Context) (RelayInventory, error)
	SimulateAllocation(context.Context, SimulationInput) (SimulationResult, error)
	GetConfig(context.Context) (ConfigDocument, error)
	GetConfigSchema(context.Context) (SchemaBundle, error)
	ValidateConfig(context.Context, ConfigCandidate) (ValidationResult, error)
	PlanConfig(context.Context, ConfigCandidate) (ConfigPlan, error)
	ApplyConfig(context.Context, ApplyRequest) (Operation, error)
	Operation(context.Context, string) (Operation, error)
	ConfigHistory(context.Context) ([]ConfigRevision, error)
	RollbackConfig(context.Context, RollbackRequest) (Operation, error)
	ReloadRuntime(context.Context, RuntimeReloadRequest) (ActivationAck, error)
}
