package servercontrolregistry

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

const (
	SchemaVersion = 1

	PurposeControlAgent = "control-agent"
	PurposeRelay        = "relay"

	ActionPair   = "pair"
	ActionAdopt  = "adopt"
	ActionRotate = "rotate"
	ActionEnroll = "enroll"

	StatePending = "pending_claim"
	StateBound   = "claiming"
	StateClaimed = "claimed"
	StateRevoked = "revoked"
	StateExpired = "expired"
)

var (
	ErrNotFound           = errors.New("server-control registry record not found")
	ErrConflict           = errors.New("server-control registry conflict")
	ErrExpired            = errors.New("pairing enrollment expired")
	ErrRevoked            = errors.New("pairing enrollment revoked")
	ErrSecret             = errors.New("pairing secret rejected")
	ErrBinding            = errors.New("pairing claim binding rejected")
	ErrRecoveryWindow     = errors.New("pairing recovery window closed")
	ErrIdentityClone      = errors.New("server-control registry host identity mismatch")
	ErrFutureSchema       = errors.New("server-control registry schema is newer than this binary")
	ErrUnsafePermissions  = errors.New("unsafe server-control registry permissions")
	ErrUnsupportedPurpose = errors.New("unsupported pairing purpose or action")
)

// PairingCodePayload is the exact canonical SP1 v1 payload. Field order is
// intentional because SP1 encodes the compact JSON bytes directly.
type PairingCodePayload struct {
	Version             int    `json:"version"`
	Purpose             string `json:"purpose"`
	BrokerOrigin        string `json:"broker_origin"`
	BrokerSPKISHA256    string `json:"broker_spki_sha256"`
	EnrollmentID        string `json:"enrollment_id"`
	ConfigurationDigest string `json:"configuration_digest"`
	ExpiresAtUnix       int64  `json:"expires_at_unix"`
	Secret              string `json:"secret"`
}

func (p PairingCodePayload) Encode() (string, error) {
	if p.Version != 1 || !validPurpose(p.Purpose) || !validSHA256(p.BrokerSPKISHA256) || !validSHA256(p.ConfigurationDigest) {
		return "", errors.New("invalid SP1 payload")
	}
	secret, err := base64.RawURLEncoding.DecodeString(p.Secret)
	if err != nil || len(secret) != 32 || p.ExpiresAtUnix < 1 {
		return "", errors.New("invalid SP1 secret or expiry")
	}
	raw, err := json.Marshal(p)
	if err != nil {
		return "", err
	}
	return "SP1." + base64.RawURLEncoding.EncodeToString(raw), nil
}

type ClaimRequest struct {
	Version             int     `json:"version"`
	Purpose             string  `json:"purpose"`
	Action              string  `json:"action"`
	EnrollmentID        string  `json:"enrollment_id"`
	ConfigurationDigest string  `json:"configuration_digest"`
	Secret              string  `json:"secret"`
	RequestDigest       string  `json:"request_digest"`
	KeyFingerprint      string  `json:"key_fingerprint"`
	CSRPEM              string  `json:"csr_pem"`
	InstanceID          *string `json:"instance_id,omitempty"`
}

type ClaimResponse struct {
	Version             int         `json:"version"`
	Purpose             string      `json:"purpose"`
	EnrollmentID        string      `json:"enrollment_id"`
	ConfigurationDigest string      `json:"configuration_digest"`
	RequestDigest       string      `json:"request_digest"`
	KeyFingerprint      string      `json:"key_fingerprint"`
	Bundle              interface{} `json:"bundle"`
}

type ControlAgentBundle struct {
	InstanceID               string                 `json:"instance_id"`
	AgentOrigin              string                 `json:"agent_origin"`
	ServerCertificatePEM     string                 `json:"server_certificate_pem"`
	ClientCAPEM              string                 `json:"client_ca_pem"`
	AllowedClientURISANs     []string               `json:"allowed_client_uri_sans"`
	ServiceJWKS              map[string]interface{} `json:"service_jwks"`
	ServiceJWTIssuer         string                 `json:"service_jwt_issuer"`
	ServiceJWTAudiencePrefix string                 `json:"service_jwt_audience_prefix"`
	CenterPublicKey          string                 `json:"center_public_key"`
}

type EnrollmentCreate struct {
	EnrollmentID        string
	Purpose             string
	Action              string
	ManagedID           string
	Name                string
	AgentOriginID       string
	AgentOrigin         string
	TLSServerName       string
	TargetInstanceID    string
	ConfigurationDigest string
	SecretDigest        string
	ExpiresAt           time.Time
	RecoveryTTL         time.Duration
	RelaySpecJSON       string
}

type Enrollment struct {
	EnrollmentID        string `json:"enrollment_id"`
	Purpose             string `json:"purpose"`
	Action              string `json:"action"`
	ManagedID           string `json:"managed_id,omitempty"`
	Name                string `json:"name,omitempty"`
	AgentOriginID       string `json:"agent_origin_id,omitempty"`
	AgentOrigin         string `json:"agent_origin,omitempty"`
	TLSServerName       string `json:"tls_server_name,omitempty"`
	TargetInstanceID    string `json:"target_instance_id,omitempty"`
	ConfigurationDigest string `json:"configuration_digest"`
	SecretDigest        string `json:"-"`
	ExpiresAtUnix       int64  `json:"expires_at_unix"`
	RecoveryTTLSeconds  int64  `json:"-"`
	State               string `json:"state"`
	RequestDigest       string `json:"request_digest,omitempty"`
	KeyFingerprint      string `json:"key_fingerprint,omitempty"`
	CSRDigest           string `json:"-"`
	InstanceUUID        string `json:"instance_id,omitempty"`
	RecoveryUntilUnix   int64  `json:"recovery_until_unix,omitempty"`
	RelaySpecJSON       string `json:"-"`
	CreatedAtUnix       int64  `json:"created_at_unix"`
	UpdatedAtUnix       int64  `json:"updated_at_unix"`
}

type ClaimBinding struct {
	Enrollment Enrollment
	Reused     bool
}

type ManagedInstance struct {
	ManagedID          string `json:"id"`
	Name               string `json:"name"`
	InstanceUUID       string `json:"instance_id"`
	AgentOrigin        string `json:"agent_origin"`
	TLSServerName      string `json:"tls_server_name"`
	CAFile             string `json:"ca_file"`
	ClientCertFile     string `json:"client_cert_file"`
	ClientKeyFile      string `json:"client_key_file"`
	ControlKeyFile     string `json:"control_key_file"`
	ControlKeyID       string `json:"control_key_id"`
	ControlIssuer      string `json:"control_issuer"`
	AuthorizedParty    string `json:"authorized_party"`
	CertificateSHA256  string `json:"certificate_sha256"`
	ControlKeySHA256   string `json:"control_key_sha256"`
	ReadOnly           bool   `json:"read_only"`
	State              string `json:"state"`
	RegistryGeneration uint64 `json:"registry_generation"`
	CreatedAtUnix      int64  `json:"created_at_unix"`
	UpdatedAtUnix      int64  `json:"updated_at_unix"`
}

type Metadata struct {
	SchemaVersion   int    `json:"schema_version"`
	Generation      uint64 `json:"generation"`
	InstallationID  string `json:"installation_id"`
	HostFingerprint string `json:"host_fingerprint"`
	Root            string `json:"root"`
}

func SecretDigest(secret string) (string, error) {
	raw, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil || len(raw) != 32 || base64.RawURLEncoding.EncodeToString(raw) != secret {
		return "", errors.New("SP1 secret must be canonical base64url for 32 bytes")
	}
	digest := sha256.Sum256(raw)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func CSRDigest(csrPEM string) string {
	digest := sha256.Sum256([]byte(csrPEM))
	return "sha256:" + hex.EncodeToString(digest[:])
}

// ExpectedRequestDigest implements the frozen SP1 v1 binding algorithm.
func ExpectedRequestDigest(request ClaimRequest) string {
	instanceID := ""
	if request.InstanceID != nil {
		instanceID = *request.InstanceID
	}
	csrDigest := sha256.Sum256([]byte(request.CSRPEM))
	csrDigestValue := "sha256:" + hex.EncodeToString(csrDigest[:])
	canonical := fmt.Sprintf("starry-pairing-claim-v1\n%s\n%s\n%s\n%s\n%s\n%s\n%s",
		request.Purpose,
		request.Action,
		request.EnrollmentID,
		request.ConfigurationDigest,
		request.KeyFingerprint,
		csrDigestValue,
		instanceID,
	)
	digest := sha256.Sum256([]byte(canonical))
	return "sha256:" + hex.EncodeToString(digest[:])
}

func ValidClaimShape(request ClaimRequest) error {
	if request.Version != 1 || !validPurposeAction(request.Purpose, request.Action) {
		return ErrUnsupportedPurpose
	}
	if request.EnrollmentID == "" || !validSHA256(request.ConfigurationDigest) || !validSHA256(request.RequestDigest) || !validSHA256(request.KeyFingerprint) {
		return ErrBinding
	}
	if len(request.CSRPEM) < 1 || len(request.CSRPEM) > 32768 {
		return ErrBinding
	}
	if _, err := SecretDigest(request.Secret); err != nil {
		return ErrSecret
	}
	if request.Purpose == PurposeControlAgent && (request.InstanceID == nil || strings.TrimSpace(*request.InstanceID) == "") {
		return ErrBinding
	}
	if request.Purpose == PurposeRelay && request.InstanceID != nil {
		return ErrBinding
	}
	if ExpectedRequestDigest(request) != request.RequestDigest {
		return ErrBinding
	}
	return nil
}

func validSHA256(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	_, err := hex.DecodeString(value[7:])
	return err == nil && strings.ToLower(value) == value
}

func validPurpose(value string) bool {
	return value == PurposeControlAgent || value == PurposeRelay
}

func validPurposeAction(purpose, action string) bool {
	if purpose == PurposeControlAgent {
		return action == ActionPair || action == ActionAdopt || action == ActionRotate
	}
	return purpose == PurposeRelay && action == ActionEnroll
}
