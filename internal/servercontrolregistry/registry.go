package servercontrolregistry

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
)

const metadataRow = 1

type OpenOptions struct {
	// HostIdentity is test/deployment input used only to derive a non-secret
	// clone-detection fingerprint. Production callers should instead provide
	// HostIdentityFile, mounted from the host when running in a container.
	HostIdentity     string
	HostIdentityFile string
	Now              func() time.Time
}

type Store struct {
	root            string
	databasePath    string
	lockPath        string
	hostFingerprint string
	db              *sql.DB
	now             func() time.Time
}

// Open explicitly initializes a registry when none exists. Long-running
// services and read-only preflight commands must use OpenExisting so a lost
// volume or changed path cannot silently create a replacement identity.
func Open(root string, options OpenOptions) (*Store, error) {
	resolved, err := prepareRoot(root)
	if err != nil {
		return nil, err
	}
	return openRegistry(resolved, options, true)
}

// OpenExisting opens only a complete existing registry. It never creates the
// root, required subdirectories, database, or a replacement installation ID.
func OpenExisting(root string, options OpenOptions) (*Store, error) {
	resolved, err := validateExistingRegistryRoot(root)
	if err != nil {
		return nil, err
	}
	return openRegistry(resolved, options, false)
}

func openRegistry(resolved string, options OpenOptions, initialize bool) (*Store, error) {
	hostFingerprint, err := deriveHostFingerprint(options.HostIdentity, options.HostIdentityFile)
	if err != nil {
		return nil, err
	}
	databasePath := filepath.Join(resolved, "registry-v1.sqlite")
	if initialize {
		if err := prepareSQLiteDatabase(databasePath); err != nil {
			return nil, err
		}
	} else if err := validateSQLiteFiles(databasePath); err != nil {
		return nil, err
	}
	db, err := openSQLite(databasePath)
	if err != nil {
		return nil, err
	}
	store := &Store{
		root:            resolved,
		databasePath:    databasePath,
		lockPath:        filepath.Join(resolved, "registry-v1.lock"),
		hostFingerprint: hostFingerprint,
		db:              db,
		now:             options.Now,
	}
	if store.now == nil {
		store.now = time.Now
	}
	if !initialize {
		// Reject an empty or unrelated database before creating a lock file.
		if err := store.validateExisting(); err != nil {
			_ = db.Close()
			return nil, err
		}
	}
	preflight := store.validateExisting
	if initialize {
		preflight = store.initialize
	}
	if err := withFileLock(context.Background(), store.lockPath, preflight); err != nil {
		_ = db.Close()
		return nil, err
	}
	if err := protectSQLiteFiles(databasePath); err != nil {
		_ = db.Close()
		return nil, err
	}
	return store, nil
}

// AdoptHost is the only supported way to change clone detection after a
// stopped-host migration. Callers must require an exact installation-id
// confirmation before invoking it; the database is otherwise left untouched.
func AdoptHost(ctx context.Context, root, expectedInstallationID string, options OpenOptions) error {
	if _, err := uuid.Parse(expectedInstallationID); err != nil {
		return errors.New("expected installation id must be a UUID")
	}
	resolved, err := validateExistingRegistryRoot(root)
	if err != nil {
		return err
	}
	fingerprint, err := deriveHostFingerprint(options.HostIdentity, options.HostIdentityFile)
	if err != nil {
		return err
	}
	databasePath := filepath.Join(resolved, "registry-v1.sqlite")
	if err := validateSQLiteFiles(databasePath); err != nil {
		return err
	}
	db, err := openSQLite(databasePath)
	if err != nil {
		return err
	}
	defer db.Close()
	return withFileLock(ctx, filepath.Join(resolved, "registry-v1.lock"), func() error {
		var schemaVersion int
		var installationID string
		if err := db.QueryRowContext(ctx, `SELECT schema_version, installation_id FROM registry_meta WHERE singleton = ?`, metadataRow).Scan(&schemaVersion, &installationID); err != nil {
			return err
		}
		if schemaVersion > SchemaVersion {
			return ErrFutureSchema
		}
		if schemaVersion != SchemaVersion || installationID != expectedInstallationID {
			return ErrBinding
		}
		result, err := db.ExecContext(ctx, `UPDATE registry_meta SET host_fingerprint = ?, generation = generation + 1, updated_at_unix = ? WHERE singleton = ? AND installation_id = ?`, fingerprint, time.Now().UTC().Unix(), metadataRow, expectedInstallationID)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil || rows != 1 {
			return ErrConflict
		}
		return protectSQLiteFiles(databasePath)
	})
}

// Purge permanently removes an explicitly identified registry and every
// credential below it. It is deliberately separate from Close and package
// removal. Callers must stop all processes sharing the registry and provide
// two independent confirmations; a wrong path or installation identity fails
// before anything is renamed or removed.
func Purge(ctx context.Context, root, expectedInstallationID, confirmation string, serviceStopped, dataLossUnderstood bool) error {
	if _, err := uuid.Parse(expectedInstallationID); err != nil {
		return errors.New("expected installation id must be a UUID")
	}
	if !serviceStopped || !dataLossUnderstood || confirmation != "confirm:purge:"+expectedInstallationID {
		return errors.New("explicit stopped-service and data-loss confirmations are required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	resolved, err := validateExistingRoot(root)
	if err != nil {
		return err
	}
	databasePath := filepath.Join(resolved, "registry-v1.sqlite")
	if err := validateSQLiteFiles(databasePath); err != nil {
		return err
	}
	db, err := openSQLite(databasePath)
	if err != nil {
		return err
	}
	databaseOpen := true
	defer func() {
		if databaseOpen {
			_ = db.Close()
		}
	}()
	tombstone := resolved + ".purged-" + expectedInstallationID
	if _, err := os.Lstat(tombstone); !errors.Is(err, os.ErrNotExist) {
		if err == nil {
			return errors.New("refusing to overwrite an existing purge tombstone")
		}
		return err
	}
	err = withFileLock(ctx, filepath.Join(resolved, "registry-v1.lock"), func() error {
		var schemaVersion int
		var installationID string
		if err := db.QueryRowContext(ctx, `SELECT schema_version, installation_id FROM registry_meta WHERE singleton = ?`, metadataRow).Scan(&schemaVersion, &installationID); err != nil {
			return err
		}
		if schemaVersion != SchemaVersion || installationID != expectedInstallationID {
			return ErrBinding
		}
		if err := db.Close(); err != nil {
			return err
		}
		databaseOpen = false
		return os.Rename(resolved, tombstone)
	})
	if err != nil {
		return err
	}
	if err := os.RemoveAll(tombstone); err != nil {
		return fmt.Errorf("registry was detached but purge cleanup failed at %s: %w", tombstone, err)
	}
	return nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func (s *Store) Root() string {
	if s == nil {
		return ""
	}
	return s.root
}

func (s *Store) Metadata(ctx context.Context) (Metadata, error) {
	if s == nil || s.db == nil {
		return Metadata{}, errors.New("server-control registry is not open")
	}
	var result Metadata
	err := s.db.QueryRowContext(ctx, `SELECT schema_version, generation, installation_id, host_fingerprint FROM registry_meta WHERE singleton = ?`, metadataRow).
		Scan(&result.SchemaVersion, &result.Generation, &result.InstallationID, &result.HostFingerprint)
	result.Root = s.root
	return result, err
}

func (s *Store) CreateEnrollment(ctx context.Context, input EnrollmentCreate) (Enrollment, error) {
	if err := validateEnrollmentCreate(input); err != nil {
		return Enrollment{}, err
	}
	now := s.now().UTC()
	if !input.ExpiresAt.After(now) {
		return Enrollment{}, ErrExpired
	}
	recoveryTTL := input.RecoveryTTL
	if recoveryTTL <= 0 || recoveryTTL > 10*time.Minute {
		recoveryTTL = 10 * time.Minute
	}
	result := Enrollment{
		EnrollmentID: input.EnrollmentID, Purpose: input.Purpose, Action: input.Action,
		ManagedID: input.ManagedID, Name: input.Name, AgentOriginID: input.AgentOriginID,
		AgentOrigin: input.AgentOrigin, TLSServerName: input.TLSServerName,
		TargetInstanceID: input.TargetInstanceID, ConfigurationDigest: input.ConfigurationDigest,
		SecretDigest: input.SecretDigest, ExpiresAtUnix: input.ExpiresAt.UTC().Unix(),
		RecoveryTTLSeconds: int64(recoveryTTL.Seconds()), State: StatePending,
		RelaySpecJSON: input.RelaySpecJSON, CreatedAtUnix: now.Unix(), UpdatedAtUnix: now.Unix(),
	}
	err := s.mutate(ctx, func(tx *sql.Tx) error {
		if input.Purpose == PurposeControlAgent {
			// A one-time code must stop reserving its managed ID once the
			// initial claim or exact-response recovery window has closed. Keep
			// the record for audit/replay rejection, but release the uniqueness
			// guard atomically with creation of the replacement enrollment.
			if _, err := tx.ExecContext(ctx, `UPDATE pairing_enrollments
				SET state = ?, updated_at_unix = ?
				WHERE purpose = ? AND managed_id = ? AND (
					(state = ? AND expires_at_unix <= ?) OR
					(state = ? AND recovery_until_unix <= ?)
				)`, StateExpired, now.Unix(), PurposeControlAgent, input.ManagedID,
				StatePending, now.Unix(), StateBound, now.Unix()); err != nil {
				return err
			}
			var active int
			if err := tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM pairing_enrollments WHERE managed_id = ? AND state IN (?, ?)`, input.ManagedID, StatePending, StateBound).Scan(&active); err != nil {
				return err
			}
			if active != 0 {
				return fmt.Errorf("%w: managed instance already has a pending claim", ErrConflict)
			}
			var managedUUID string
			err := tx.QueryRowContext(ctx, `SELECT instance_uuid FROM managed_instances WHERE managed_id = ?`, input.ManagedID).Scan(&managedUUID)
			switch input.Action {
			case ActionPair, ActionAdopt:
				if err == nil {
					return fmt.Errorf("%w: managed instance already exists", ErrConflict)
				}
				if !errors.Is(err, sql.ErrNoRows) {
					return err
				}
			case ActionRotate:
				if errors.Is(err, sql.ErrNoRows) {
					return fmt.Errorf("%w: managed instance does not exist", ErrConflict)
				}
				if err != nil {
					return err
				}
				if input.TargetInstanceID != managedUUID {
					return fmt.Errorf("%w: rotation instance binding changed", ErrBinding)
				}
			}
		}
		_, err := tx.ExecContext(ctx, `INSERT INTO pairing_enrollments (
			enrollment_id, purpose, action, managed_id, name, agent_origin_id, agent_origin, tls_server_name,
			target_instance_id, configuration_digest, secret_digest, expires_at_unix, recovery_ttl_seconds,
			state, request_digest, key_fingerprint, csr_digest, instance_uuid, recovery_until_unix,
			relay_spec_json, created_at_unix, updated_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, '', '', '', '', 0, ?, ?, ?)`,
			result.EnrollmentID, result.Purpose, result.Action, result.ManagedID, result.Name, result.AgentOriginID,
			result.AgentOrigin, result.TLSServerName, result.TargetInstanceID, result.ConfigurationDigest,
			result.SecretDigest, result.ExpiresAtUnix, result.RecoveryTTLSeconds, result.State,
			result.RelaySpecJSON, result.CreatedAtUnix, result.UpdatedAtUnix)
		return err
	})
	return result, err
}

func (s *Store) BeginClaim(ctx context.Context, request ClaimRequest) (ClaimBinding, error) {
	if err := ValidClaimShape(request); err != nil {
		return ClaimBinding{}, err
	}
	if _, err := uuid.Parse(request.EnrollmentID); err != nil {
		return ClaimBinding{}, ErrBinding
	}
	if request.InstanceID != nil {
		if _, err := uuid.Parse(*request.InstanceID); err != nil {
			return ClaimBinding{}, ErrBinding
		}
	}
	secretDigest, err := SecretDigest(request.Secret)
	if err != nil {
		return ClaimBinding{}, ErrSecret
	}
	var result ClaimBinding
	err = s.mutateConditional(ctx, func(tx *sql.Tx) (bool, error) {
		enrollment, err := loadEnrollment(ctx, tx, request.EnrollmentID)
		if err != nil {
			return false, err
		}
		nowUnix := s.now().UTC().Unix()
		if enrollment.State == StateRevoked {
			return false, ErrRevoked
		}
		if enrollment.State == StateExpired || (enrollment.State == StatePending && nowUnix >= enrollment.ExpiresAtUnix) {
			return false, ErrExpired
		}
		if enrollment.Purpose != request.Purpose || enrollment.Action != request.Action || enrollment.ConfigurationDigest != request.ConfigurationDigest {
			return false, ErrBinding
		}
		if !hmac.Equal([]byte(enrollment.SecretDigest), []byte(secretDigest)) {
			return false, ErrSecret
		}
		instanceUUID := ""
		if request.InstanceID != nil {
			instanceUUID = *request.InstanceID
		}
		if enrollment.TargetInstanceID != "" && enrollment.TargetInstanceID != instanceUUID {
			return false, ErrBinding
		}
		csrDigest := CSRDigest(request.CSRPEM)
		switch enrollment.State {
		case StatePending:
			recoveryUntil := nowUnix + enrollment.RecoveryTTLSeconds
			updated, err := tx.ExecContext(ctx, `UPDATE pairing_enrollments SET state = ?, request_digest = ?, key_fingerprint = ?, csr_digest = ?, instance_uuid = ?, recovery_until_unix = ?, updated_at_unix = ? WHERE enrollment_id = ? AND state = ?`,
				StateBound, request.RequestDigest, request.KeyFingerprint, csrDigest, instanceUUID, recoveryUntil, nowUnix, request.EnrollmentID, StatePending)
			if err != nil {
				return false, err
			}
			rows, err := updated.RowsAffected()
			if err != nil || rows != 1 {
				return false, ErrConflict
			}
			enrollment.State = StateBound
			enrollment.RequestDigest = request.RequestDigest
			enrollment.KeyFingerprint = request.KeyFingerprint
			enrollment.CSRDigest = csrDigest
			enrollment.InstanceUUID = instanceUUID
			enrollment.RecoveryUntilUnix = recoveryUntil
			enrollment.UpdatedAtUnix = nowUnix
			result = ClaimBinding{Enrollment: enrollment}
			return true, nil
		case StateBound, StateClaimed:
			if enrollment.RecoveryUntilUnix <= nowUnix {
				return false, ErrRecoveryWindow
			}
			if enrollment.RequestDigest != request.RequestDigest || enrollment.KeyFingerprint != request.KeyFingerprint || enrollment.CSRDigest != csrDigest || enrollment.InstanceUUID != instanceUUID {
				return false, ErrBinding
			}
			result = ClaimBinding{Enrollment: enrollment, Reused: true}
			return false, nil
		default:
			return false, ErrBinding
		}
	})
	return result, err
}

func (s *Store) CompleteControlClaim(ctx context.Context, enrollmentID string, input ManagedInstance) (ManagedInstance, error) {
	if input.ManagedID == "" || input.InstanceUUID == "" || input.AgentOrigin == "" || !input.ReadOnly || input.State != "paired_read_only" {
		return ManagedInstance{}, errors.New("managed control instance must be complete and initially read-only")
	}
	nowUnix := s.now().UTC().Unix()
	result := input
	err := s.mutateConditional(ctx, func(tx *sql.Tx) (bool, error) {
		enrollment, err := loadEnrollment(ctx, tx, enrollmentID)
		if err != nil {
			return false, err
		}
		if enrollment.Purpose != PurposeControlAgent || (enrollment.State != StateBound && enrollment.State != StateClaimed) || enrollment.ManagedID != input.ManagedID || enrollment.InstanceUUID != input.InstanceUUID {
			return false, ErrBinding
		}
		if enrollment.State == StateClaimed {
			existing, err := loadManagedInstance(ctx, tx, input.ManagedID)
			if err != nil || !sameManagedIdentity(existing, input) {
				return false, ErrBinding
			}
			result = existing
			return false, nil
		}
		var generation uint64
		if err := tx.QueryRowContext(ctx, `SELECT generation + 1 FROM registry_meta WHERE singleton = ?`, metadataRow).Scan(&generation); err != nil {
			return false, err
		}
		result.RegistryGeneration = generation
		result.UpdatedAtUnix = nowUnix
		if result.CreatedAtUnix == 0 {
			result.CreatedAtUnix = nowUnix
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO managed_instances (
			managed_id, name, instance_uuid, agent_origin, tls_server_name, ca_file, client_cert_file,
			client_key_file, control_key_file, control_key_id, control_issuer, authorized_party,
			certificate_sha256, control_key_sha256, read_only, state, registry_generation, created_at_unix, updated_at_unix
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 1, ?, ?, ?, ?)
		ON CONFLICT(managed_id) DO UPDATE SET name=excluded.name, instance_uuid=excluded.instance_uuid,
			agent_origin=excluded.agent_origin, tls_server_name=excluded.tls_server_name, ca_file=excluded.ca_file,
			client_cert_file=excluded.client_cert_file, client_key_file=excluded.client_key_file,
			control_key_file=excluded.control_key_file, control_key_id=excluded.control_key_id,
			control_issuer=excluded.control_issuer, authorized_party=excluded.authorized_party,
			certificate_sha256=excluded.certificate_sha256, control_key_sha256=excluded.control_key_sha256,
			read_only=1, state=excluded.state, registry_generation=excluded.registry_generation, updated_at_unix=excluded.updated_at_unix`,
			result.ManagedID, result.Name, result.InstanceUUID, result.AgentOrigin, result.TLSServerName,
			result.CAFile, result.ClientCertFile, result.ClientKeyFile, result.ControlKeyFile, result.ControlKeyID,
			result.ControlIssuer, result.AuthorizedParty, result.CertificateSHA256, result.ControlKeySHA256,
			result.State, result.RegistryGeneration, result.CreatedAtUnix, result.UpdatedAtUnix)
		if err != nil {
			return false, err
		}
		_, err = tx.ExecContext(ctx, `UPDATE pairing_enrollments SET state = ?, updated_at_unix = ? WHERE enrollment_id = ?`, StateClaimed, nowUnix, enrollmentID)
		return err == nil, err
	})
	return result, err
}

func (s *Store) CompleteRelayClaim(ctx context.Context, enrollmentID string) error {
	return s.mutateConditional(ctx, func(tx *sql.Tx) (bool, error) {
		enrollment, err := loadEnrollment(ctx, tx, enrollmentID)
		if err != nil {
			return false, err
		}
		if enrollment.Purpose != PurposeRelay || (enrollment.State != StateBound && enrollment.State != StateClaimed) {
			return false, ErrBinding
		}
		if enrollment.State == StateClaimed {
			return false, nil
		}
		_, err = tx.ExecContext(ctx, `UPDATE pairing_enrollments SET state = ?, updated_at_unix = ? WHERE enrollment_id = ?`, StateClaimed, s.now().UTC().Unix(), enrollmentID)
		return err == nil, err
	})
}

func (s *Store) RevokeEnrollment(ctx context.Context, enrollmentID string) error {
	return s.mutate(ctx, func(tx *sql.Tx) error {
		result, err := tx.ExecContext(ctx, `UPDATE pairing_enrollments SET state = ?, updated_at_unix = ? WHERE enrollment_id = ? AND state IN (?, ?)`, StateRevoked, s.now().UTC().Unix(), enrollmentID, StatePending, StateBound)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return ErrNotFound
		}
		return nil
	})
}

func (s *Store) Enrollment(ctx context.Context, enrollmentID string) (Enrollment, error) {
	return loadEnrollment(ctx, s.db, enrollmentID)
}

func (s *Store) ManagedInstances(ctx context.Context) ([]ManagedInstance, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT managed_id, name, instance_uuid, agent_origin, tls_server_name,
		ca_file, client_cert_file, client_key_file, control_key_file, control_key_id, control_issuer,
		authorized_party, certificate_sha256, control_key_sha256, read_only, state, registry_generation,
		created_at_unix, updated_at_unix FROM managed_instances ORDER BY managed_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ManagedInstance
	for rows.Next() {
		var item ManagedInstance
		if err := rows.Scan(&item.ManagedID, &item.Name, &item.InstanceUUID, &item.AgentOrigin, &item.TLSServerName,
			&item.CAFile, &item.ClientCertFile, &item.ClientKeyFile, &item.ControlKeyFile, &item.ControlKeyID,
			&item.ControlIssuer, &item.AuthorizedParty, &item.CertificateSHA256, &item.ControlKeySHA256,
			&item.ReadOnly, &item.State, &item.RegistryGeneration, &item.CreatedAtUnix, &item.UpdatedAtUnix); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (s *Store) ManagedInstance(ctx context.Context, managedID string) (ManagedInstance, error) {
	return loadManagedInstance(ctx, s.db, managedID)
}

func loadManagedInstance(ctx context.Context, query rowQuery, managedID string) (ManagedInstance, error) {
	var item ManagedInstance
	err := query.QueryRowContext(ctx, `SELECT managed_id, name, instance_uuid, agent_origin, tls_server_name,
		ca_file, client_cert_file, client_key_file, control_key_file, control_key_id, control_issuer,
		authorized_party, certificate_sha256, control_key_sha256, read_only, state, registry_generation,
		created_at_unix, updated_at_unix FROM managed_instances WHERE managed_id = ?`, managedID).
		Scan(&item.ManagedID, &item.Name, &item.InstanceUUID, &item.AgentOrigin, &item.TLSServerName,
			&item.CAFile, &item.ClientCertFile, &item.ClientKeyFile, &item.ControlKeyFile, &item.ControlKeyID,
			&item.ControlIssuer, &item.AuthorizedParty, &item.CertificateSHA256, &item.ControlKeySHA256,
			&item.ReadOnly, &item.State, &item.RegistryGeneration, &item.CreatedAtUnix, &item.UpdatedAtUnix)
	if errors.Is(err, sql.ErrNoRows) {
		return ManagedInstance{}, ErrNotFound
	}
	return item, err
}

func sameManagedIdentity(left, right ManagedInstance) bool {
	return left.ManagedID == right.ManagedID && left.Name == right.Name && left.InstanceUUID == right.InstanceUUID &&
		left.AgentOrigin == right.AgentOrigin && left.TLSServerName == right.TLSServerName && left.CAFile == right.CAFile &&
		left.ClientCertFile == right.ClientCertFile && left.ClientKeyFile == right.ClientKeyFile &&
		left.ControlKeyFile == right.ControlKeyFile && left.ControlKeyID == right.ControlKeyID &&
		left.ControlIssuer == right.ControlIssuer && left.AuthorizedParty == right.AuthorizedParty &&
		left.CertificateSHA256 == right.CertificateSHA256 && left.ControlKeySHA256 == right.ControlKeySHA256
}

func (s *Store) SetManagedReadOnly(ctx context.Context, managedID string, readOnly bool) (ManagedInstance, error) {
	nowUnix := s.now().UTC().Unix()
	err := s.mutate(ctx, func(tx *sql.Tx) error {
		state := "paired_write_enabled"
		if readOnly {
			state = "paired_read_only"
		}
		result, err := tx.ExecContext(ctx, `UPDATE managed_instances SET read_only = ?, state = ?, registry_generation = (SELECT generation + 1 FROM registry_meta WHERE singleton = ?), updated_at_unix = ? WHERE managed_id = ?`, readOnly, state, metadataRow, nowUnix, managedID)
		if err != nil {
			return err
		}
		rows, err := result.RowsAffected()
		if err != nil {
			return err
		}
		if rows != 1 {
			return ErrNotFound
		}
		return nil
	})
	if err != nil {
		return ManagedInstance{}, err
	}
	return s.ManagedInstance(ctx, managedID)
}

func (s *Store) initialize() error {
	ctx := context.Background()
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='registry_meta'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		var otherTables int
		if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'`).Scan(&otherTables); err != nil {
			return err
		}
		if otherTables != 0 {
			return errors.New("refusing to initialize a non-empty database without registry metadata")
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		if _, err := tx.ExecContext(ctx, registrySchema); err != nil {
			return fmt.Errorf("create server-control registry schema: %w", err)
		}
		installationID, err := uuid.NewV7()
		if err != nil {
			installationID = uuid.New()
		}
		nowUnix := s.now().UTC().Unix()
		if _, err := tx.ExecContext(ctx, `INSERT INTO registry_meta (singleton, schema_version, generation, installation_id, host_fingerprint, created_at_unix, updated_at_unix) VALUES (?, ?, 1, ?, ?, ?, ?)`, metadataRow, SchemaVersion, installationID.String(), s.hostFingerprint, nowUnix, nowUnix); err != nil {
			return err
		}
		return tx.Commit()
	}
	return s.validateExisting()
}

func (s *Store) validateExisting() error {
	ctx := context.Background()
	var exists int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sqlite_master WHERE type='table' AND name='registry_meta'`).Scan(&exists); err != nil {
		return err
	}
	if exists == 0 {
		return fmt.Errorf("%w: registry metadata is absent", ErrNotFound)
	}
	metadata, err := s.Metadata(ctx)
	if err != nil {
		return fmt.Errorf("read server-control registry metadata: %w", err)
	}
	if metadata.SchemaVersion > SchemaVersion {
		return fmt.Errorf("%w: database=%d binary=%d", ErrFutureSchema, metadata.SchemaVersion, SchemaVersion)
	}
	if metadata.SchemaVersion != SchemaVersion {
		return fmt.Errorf("unsupported server-control registry schema %d", metadata.SchemaVersion)
	}
	if metadata.HostFingerprint != s.hostFingerprint {
		return fmt.Errorf("%w: installation %s must be explicitly adopted after the old host is stopped", ErrIdentityClone, metadata.InstallationID)
	}
	return nil
}

func (s *Store) mutate(ctx context.Context, operation func(*sql.Tx) error) error {
	return s.mutateConditional(ctx, func(tx *sql.Tx) (bool, error) {
		if err := operation(tx); err != nil {
			return false, err
		}
		return true, nil
	})
}

func (s *Store) mutateConditional(ctx context.Context, operation func(*sql.Tx) (bool, error)) error {
	if s == nil || s.db == nil {
		return errors.New("server-control registry is not open")
	}
	return withFileLock(ctx, s.lockPath, func() error {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		defer tx.Rollback()
		changed, err := operation(tx)
		if err != nil {
			return err
		}
		if changed {
			if _, err := tx.ExecContext(ctx, `UPDATE registry_meta SET generation = generation + 1, updated_at_unix = ? WHERE singleton = ?`, s.now().UTC().Unix(), metadataRow); err != nil {
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		return protectSQLiteFiles(s.databasePath)
	})
}

type rowQuery interface {
	QueryRowContext(context.Context, string, ...interface{}) *sql.Row
}

func loadEnrollment(ctx context.Context, query rowQuery, enrollmentID string) (Enrollment, error) {
	var item Enrollment
	err := query.QueryRowContext(ctx, `SELECT enrollment_id, purpose, action, managed_id, name, agent_origin_id,
		agent_origin, tls_server_name, target_instance_id, configuration_digest, secret_digest, expires_at_unix,
		recovery_ttl_seconds, state, request_digest, key_fingerprint, csr_digest, instance_uuid,
		recovery_until_unix, relay_spec_json, created_at_unix, updated_at_unix
		FROM pairing_enrollments WHERE enrollment_id = ?`, enrollmentID).
		Scan(&item.EnrollmentID, &item.Purpose, &item.Action, &item.ManagedID, &item.Name, &item.AgentOriginID,
			&item.AgentOrigin, &item.TLSServerName, &item.TargetInstanceID, &item.ConfigurationDigest,
			&item.SecretDigest, &item.ExpiresAtUnix, &item.RecoveryTTLSeconds, &item.State,
			&item.RequestDigest, &item.KeyFingerprint, &item.CSRDigest, &item.InstanceUUID,
			&item.RecoveryUntilUnix, &item.RelaySpecJSON, &item.CreatedAtUnix, &item.UpdatedAtUnix)
	if errors.Is(err, sql.ErrNoRows) {
		return Enrollment{}, ErrNotFound
	}
	return item, err
}

func validateEnrollmentCreate(input EnrollmentCreate) error {
	if _, err := uuid.Parse(input.EnrollmentID); err != nil || !validPurposeAction(input.Purpose, input.Action) || !validSHA256(input.ConfigurationDigest) || !validSHA256(input.SecretDigest) {
		return ErrBinding
	}
	if input.ExpiresAt.IsZero() || input.ExpiresAt.Unix() < 1 {
		return ErrExpired
	}
	if input.Purpose == PurposeControlAgent {
		if input.ManagedID == "" || input.Name == "" || input.AgentOriginID == "" || input.AgentOrigin == "" || input.TLSServerName == "" || input.RelaySpecJSON != "" {
			return ErrBinding
		}
	} else if input.ManagedID == "" || input.RelaySpecJSON == "" {
		return ErrBinding
	}
	return nil
}

func prepareRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("server-control registry root is required")
	}
	resolved, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	info, err := os.Lstat(resolved)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(resolved, 0o700); err != nil {
			return "", err
		}
		info, err = os.Lstat(resolved)
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 || !ownedByCurrentUser(info) {
		return "", fmt.Errorf("%w: %s must be an owner-only 0700 directory and not a symlink", ErrUnsafePermissions, resolved)
	}
	for _, name := range []string{"pki", "instances", "exports"} {
		path := filepath.Join(resolved, name)
		child, childErr := os.Lstat(path)
		if errors.Is(childErr, os.ErrNotExist) {
			if childErr = os.Mkdir(path, 0o700); childErr == nil {
				child, childErr = os.Lstat(path)
			}
		}
		if childErr != nil || !child.IsDir() || child.Mode()&os.ModeSymlink != 0 || child.Mode().Perm() != 0o700 || !ownedByCurrentUser(child) {
			return "", fmt.Errorf("%w: %s must be an owner-only 0700 directory and not a symlink", ErrUnsafePermissions, path)
		}
	}
	return resolved, nil
}

func validateExistingRegistryRoot(root string) (string, error) {
	resolved, err := validateExistingRoot(root)
	if err != nil {
		return "", err
	}
	databasePath := filepath.Join(resolved, "registry-v1.sqlite")
	if _, err := os.Lstat(databasePath); errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%w: no registry database at %s", ErrNotFound, databasePath)
	} else if err != nil {
		return "", err
	}
	for _, name := range []string{"pki", "instances", "exports"} {
		path := filepath.Join(resolved, name)
		child, childErr := os.Lstat(path)
		if childErr != nil || !child.IsDir() || child.Mode()&os.ModeSymlink != 0 || child.Mode().Perm() != 0o700 || !ownedByCurrentUser(child) {
			return "", fmt.Errorf("%w: %s must be an existing owner-only 0700 directory and not a symlink", ErrUnsafePermissions, path)
		}
	}
	if err := validateSQLiteFiles(databasePath); err != nil {
		return "", err
	}
	return resolved, nil
}

func validateExistingRoot(root string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("server-control registry root is required")
	}
	resolved, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved = filepath.Clean(resolved)
	if resolved == string(filepath.Separator) || filepath.Dir(resolved) == resolved {
		return "", errors.New("refusing to operate on a filesystem root")
	}
	info, err := os.Lstat(resolved)
	if errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("%w: no registry directory at %s", ErrNotFound, resolved)
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 || !ownedByCurrentUser(info) {
		return "", fmt.Errorf("%w: %s must be an existing owner-only 0700 directory and not a symlink", ErrUnsafePermissions, resolved)
	}
	return resolved, nil
}

func validateSecureRegularFile(path string) error {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || !ownedByCurrentUser(info) {
		return fmt.Errorf("%w: %s must be an owner-owned 0600 regular file", ErrUnsafePermissions, path)
	}
	return nil
}

func prepareSQLiteDatabase(databasePath string) error {
	if _, err := os.Lstat(databasePath); err == nil {
		return validateSQLiteFiles(databasePath)
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(databasePath, os.O_RDWR|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if err := file.Chmod(0o600); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}

func validateSQLiteFiles(databasePath string) error {
	if err := validateSecureRegularFile(databasePath); err != nil {
		return err
	}
	for _, suffix := range []string{"-wal", "-shm", "-journal"} {
		path := databasePath + suffix
		if _, err := os.Lstat(path); errors.Is(err, os.ErrNotExist) {
			continue
		} else if err != nil {
			return err
		}
		if err := validateSecureRegularFile(path); err != nil {
			return err
		}
	}
	return nil
}

func deriveHostFingerprint(identity, identityFile string) (string, error) {
	identity = strings.TrimSpace(identity)
	if identity == "" {
		if identityFile == "" {
			identityFile = "/etc/machine-id"
		}
		if !filepath.IsAbs(identityFile) || filepath.Clean(identityFile) != identityFile {
			return "", errors.New("host identity file must be an absolute clean path")
		}
		info, err := os.Lstat(identityFile)
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Size() < 1 || info.Size() > 1024 {
			return "", errors.New("host identity file must be a non-symlink regular file of at most 1024 bytes")
		}
		raw, err := os.ReadFile(identityFile)
		if err != nil {
			return "", fmt.Errorf("read host identity file for registry clone detection: %w", err)
		}
		identity = strings.TrimSpace(string(raw))
	}
	if identity == "" || len(identity) > 1024 {
		return "", errors.New("invalid host identity for registry clone detection")
	}
	digest := sha256.Sum256([]byte("kessoku-server-control-host-v1\n" + identity))
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func openSQLite(path string) (*sql.DB, error) {
	// mode=rw is deliberate: creation is a separate, permission-checked step.
	// SQLite must never manufacture a replacement registry after preflight.
	dsn := (&url.URL{Scheme: "file", Path: path}).String() + "?mode=rw&_busy_timeout=5000&_foreign_keys=on&_journal_mode=WAL&_synchronous=FULL"
	db, err := sql.Open("sqlite3", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if err := db.Ping(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return db, nil
}

func protectSQLiteFiles(databasePath string) error {
	for _, path := range []string{databasePath, databasePath + "-wal", databasePath + "-shm"} {
		info, err := os.Lstat(path)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil || !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || !ownedByCurrentUser(info) {
			return fmt.Errorf("%w: %s must be an owner-owned regular file", ErrUnsafePermissions, path)
		}
		if err := os.Chmod(path, 0o600); err != nil {
			return err
		}
	}
	return nil
}

const registrySchema = `
CREATE TABLE registry_meta (
  singleton INTEGER PRIMARY KEY CHECK(singleton = 1),
  schema_version INTEGER NOT NULL,
  generation INTEGER NOT NULL CHECK(generation >= 1),
  installation_id TEXT NOT NULL UNIQUE,
  host_fingerprint TEXT NOT NULL,
  created_at_unix INTEGER NOT NULL,
  updated_at_unix INTEGER NOT NULL
);
CREATE TABLE pairing_enrollments (
  enrollment_id TEXT PRIMARY KEY,
  purpose TEXT NOT NULL,
  action TEXT NOT NULL,
  managed_id TEXT NOT NULL,
  name TEXT NOT NULL,
  agent_origin_id TEXT NOT NULL,
  agent_origin TEXT NOT NULL,
  tls_server_name TEXT NOT NULL,
  target_instance_id TEXT NOT NULL,
  configuration_digest TEXT NOT NULL,
  secret_digest TEXT NOT NULL,
  expires_at_unix INTEGER NOT NULL,
  recovery_ttl_seconds INTEGER NOT NULL,
  state TEXT NOT NULL,
  request_digest TEXT NOT NULL,
  key_fingerprint TEXT NOT NULL,
  csr_digest TEXT NOT NULL,
  instance_uuid TEXT NOT NULL,
  recovery_until_unix INTEGER NOT NULL,
  relay_spec_json TEXT NOT NULL,
  created_at_unix INTEGER NOT NULL,
  updated_at_unix INTEGER NOT NULL
);
CREATE INDEX pairing_enrollments_managed_state ON pairing_enrollments(managed_id, state);
CREATE TABLE managed_instances (
  managed_id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  instance_uuid TEXT NOT NULL UNIQUE,
  agent_origin TEXT NOT NULL,
  tls_server_name TEXT NOT NULL,
  ca_file TEXT NOT NULL,
  client_cert_file TEXT NOT NULL,
  client_key_file TEXT NOT NULL,
  control_key_file TEXT NOT NULL,
  control_key_id TEXT NOT NULL,
  control_issuer TEXT NOT NULL,
  authorized_party TEXT NOT NULL,
  certificate_sha256 TEXT NOT NULL,
  control_key_sha256 TEXT NOT NULL,
  read_only INTEGER NOT NULL CHECK(read_only IN (0, 1)),
  state TEXT NOT NULL,
  registry_generation INTEGER NOT NULL,
  created_at_unix INTEGER NOT NULL,
  updated_at_unix INTEGER NOT NULL
);
`
