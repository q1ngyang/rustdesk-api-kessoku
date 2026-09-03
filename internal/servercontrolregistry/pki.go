package servercontrolregistry

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

const (
	serviceJWTAudiencePrefix = "urn:starry-control:"
	certificateLifetime      = 397 * 24 * time.Hour
)

var managedIDPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]{0,127}$`)

type ControlIdentityOptions struct {
	ManagedID       string
	EnrollmentID    string
	Name            string
	AgentOrigin     string
	TLSServerName   string
	BrokerOrigin    string
	CenterPublicKey string
	Now             func() time.Time
}

type PreparedControlIdentity struct {
	Directory           string
	CAFile              string
	CAKeyFile           string
	ClientCertFile      string
	ClientKeyFile       string
	ControlKeyFile      string
	AgentCertFile       string
	AuthorizedParty     string
	ControlKeyID        string
	ControlKeySHA256    string
	ConfigurationDigest string
	ServiceJWKS         map[string]interface{}
	ClientCAPEM         string
	Options             ControlIdentityOptions
}

func PrepareControlIdentity(root string, options ControlIdentityOptions) (PreparedControlIdentity, error) {
	if !managedIDPattern.MatchString(options.ManagedID) {
		return PreparedControlIdentity{}, errors.New("invalid managed instance id")
	}
	if _, err := uuid.Parse(options.EnrollmentID); err != nil {
		return PreparedControlIdentity{}, errors.New("invalid pairing enrollment id")
	}
	if options.Name == "" || options.AgentOrigin == "" || options.TLSServerName == "" || options.BrokerOrigin == "" {
		return PreparedControlIdentity{}, errors.New("incomplete control identity options")
	}
	options.CenterPublicKey = strings.TrimSpace(options.CenterPublicKey)
	if len(options.CenterPublicKey) < 43 || len(options.CenterPublicKey) > 128 {
		return PreparedControlIdentity{}, errors.New("center public key does not satisfy the frozen SP1 contract")
	}
	resolved, err := validateExistingRegistryRoot(root)
	if err != nil {
		return PreparedControlIdentity{}, err
	}
	directory := filepath.Join(resolved, "instances", options.ManagedID, options.EnrollmentID)
	if err := ensurePrivateDirectory(filepath.Dir(directory)); err != nil {
		return PreparedControlIdentity{}, err
	}
	if err := ensurePrivateDirectory(directory); err != nil {
		return PreparedControlIdentity{}, err
	}
	prepared := PreparedControlIdentity{
		Directory:       directory,
		CAFile:          filepath.Join(directory, "ca.pem"),
		CAKeyFile:       filepath.Join(directory, "ca-key.pem"),
		ClientCertFile:  filepath.Join(directory, "client-cert.pem"),
		ClientKeyFile:   filepath.Join(directory, "client-key.pem"),
		ControlKeyFile:  filepath.Join(directory, "service-jwt-key.pem"),
		AgentCertFile:   filepath.Join(directory, "agent-server-cert.pem"),
		AuthorizedParty: "spiffe://kessoku/server-control/" + options.ManagedID,
		Options:         options,
	}
	err = withFileLock(context.Background(), filepath.Join(directory, "identity.lock"), func() error {
		if err := prepareCAAndClient(&prepared); err != nil {
			return err
		}
		if err := prepareControlSigner(&prepared); err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return PreparedControlIdentity{}, err
	}
	configuration := map[string]interface{}{
		"agent_origin":                options.AgentOrigin,
		"allowed_client_uri_sans":     []string{prepared.AuthorizedParty},
		"center_public_key":           options.CenterPublicKey,
		"service_jwt_audience_prefix": serviceJWTAudiencePrefix,
		"service_jwt_issuer":          options.BrokerOrigin,
	}
	raw, err := json.Marshal(configuration)
	if err != nil {
		return PreparedControlIdentity{}, err
	}
	digest := sha256.Sum256(raw)
	prepared.ConfigurationDigest = "sha256:" + hex.EncodeToString(digest[:])
	return prepared, nil
}

func (prepared PreparedControlIdentity) IssueAgentCertificate(ctx context.Context, request ClaimRequest) (string, error) {
	if err := ValidClaimShape(request); err != nil {
		return "", err
	}
	if request.ConfigurationDigest != prepared.ConfigurationDigest {
		return "", ErrBinding
	}
	csr, err := parseAndVerifyCSR(request.CSRPEM)
	if err != nil {
		return "", err
	}
	fingerprint, err := publicKeyFingerprint(csr.PublicKey)
	if err != nil || fingerprint != request.KeyFingerprint {
		return "", ErrBinding
	}
	if !csrContainsHost(csr, prepared.Options.TLSServerName) {
		return "", errors.New("Agent CSR does not contain the pre-approved TLS server name")
	}
	var encoded string
	err = withFileLock(ctx, filepath.Join(prepared.Directory, "identity.lock"), func() error {
		if raw, readErr := readSecureRegularFile(prepared.AgentCertFile, 65536); readErr == nil {
			if validateAgentCertificate(raw, csr.PublicKey, prepared) != nil {
				return errors.New("stored Agent certificate does not match the bound CSR")
			}
			encoded = string(raw)
			return nil
		} else if !errors.Is(readErr, os.ErrNotExist) {
			return readErr
		}
		caCertificate, caKey, err := loadCA(prepared.CAFile, prepared.CAKeyFile)
		if err != nil {
			return err
		}
		now := time.Now
		if prepared.Options.Now != nil {
			now = prepared.Options.Now
		}
		serial, err := randomSerial()
		if err != nil {
			return err
		}
		template := &x509.Certificate{
			SerialNumber: serial,
			Subject:      pkix.Name{CommonName: prepared.Options.TLSServerName},
			NotBefore:    now().UTC().Add(-5 * time.Minute),
			NotAfter:     now().UTC().Add(certificateLifetime),
			KeyUsage:     x509.KeyUsageDigitalSignature,
			ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		}
		if address := net.ParseIP(prepared.Options.TLSServerName); address != nil {
			template.IPAddresses = []net.IP{address}
		} else {
			template.DNSNames = []string{prepared.Options.TLSServerName}
		}
		der, err := x509.CreateCertificate(rand.Reader, template, caCertificate, csr.PublicKey, caKey)
		if err != nil {
			return err
		}
		raw := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
		if err := atomicWritePrivate(prepared.AgentCertFile, raw); err != nil {
			return err
		}
		encoded = string(raw)
		return nil
	})
	return encoded, err
}

func (prepared PreparedControlIdentity) ManagedInstance(instanceUUID string) (ManagedInstance, error) {
	if _, err := uuid.Parse(instanceUUID); err != nil {
		return ManagedInstance{}, ErrBinding
	}
	certificate, err := readSecureRegularFile(prepared.AgentCertFile, 65536)
	if err != nil {
		return ManagedInstance{}, err
	}
	digest := sha256.Sum256(certificate)
	return ManagedInstance{
		ManagedID:         prepared.Options.ManagedID,
		Name:              prepared.Options.Name,
		InstanceUUID:      instanceUUID,
		AgentOrigin:       prepared.Options.AgentOrigin,
		TLSServerName:     prepared.Options.TLSServerName,
		CAFile:            prepared.CAFile,
		ClientCertFile:    prepared.ClientCertFile,
		ClientKeyFile:     prepared.ClientKeyFile,
		ControlKeyFile:    prepared.ControlKeyFile,
		ControlKeyID:      prepared.ControlKeyID,
		ControlIssuer:     prepared.Options.BrokerOrigin,
		AuthorizedParty:   prepared.AuthorizedParty,
		CertificateSHA256: "sha256:" + hex.EncodeToString(digest[:]),
		ControlKeySHA256:  prepared.ControlKeySHA256,
		ReadOnly:          true,
		State:             "paired_read_only",
	}, nil
}

func (prepared PreparedControlIdentity) Bundle(instanceUUID, serverCertificate string) ControlAgentBundle {
	return ControlAgentBundle{
		InstanceID:               instanceUUID,
		AgentOrigin:              prepared.Options.AgentOrigin,
		ServerCertificatePEM:     serverCertificate,
		ClientCAPEM:              prepared.ClientCAPEM,
		AllowedClientURISANs:     []string{prepared.AuthorizedParty},
		ServiceJWKS:              prepared.ServiceJWKS,
		ServiceJWTIssuer:         prepared.Options.BrokerOrigin,
		ServiceJWTAudiencePrefix: serviceJWTAudiencePrefix,
		CenterPublicKey:          prepared.Options.CenterPublicKey,
	}
}

func WriteStaticExport(root string, instance ManagedInstance) (string, error) {
	if !managedIDPattern.MatchString(instance.ManagedID) {
		return "", errors.New("invalid managed instance id")
	}
	resolved, err := validateExistingRegistryRoot(root)
	if err != nil {
		return "", err
	}
	type staticInstance struct {
		ID                 string `yaml:"id"`
		Name               string `yaml:"name"`
		Enabled            bool   `yaml:"enabled"`
		BaseURL            string `yaml:"base-url"`
		ExpectedInstanceID string `yaml:"expected-instance-id"`
		TLSServerName      string `yaml:"tls-server-name"`
		CAFile             string `yaml:"ca-file"`
		ClientCertFile     string `yaml:"client-cert-file"`
		ClientKeyFile      string `yaml:"client-key-file"`
		ControlKeyFile     string `yaml:"control-key-file"`
		ControlKeyID       string `yaml:"control-key-id"`
		ControlIssuer      string `yaml:"control-issuer"`
		AuthorizedParty    string `yaml:"authorized-party"`
	}
	document := struct {
		ServerControl struct {
			Instances []staticInstance `yaml:"instances"`
		} `yaml:"server-control"`
	}{}
	document.ServerControl.Instances = []staticInstance{{
		ID: instance.ManagedID, Name: instance.Name, Enabled: true, BaseURL: instance.AgentOrigin,
		ExpectedInstanceID: instance.InstanceUUID, TLSServerName: instance.TLSServerName,
		CAFile: instance.CAFile, ClientCertFile: instance.ClientCertFile, ClientKeyFile: instance.ClientKeyFile,
		ControlKeyFile: instance.ControlKeyFile, ControlKeyID: instance.ControlKeyID,
		ControlIssuer: instance.ControlIssuer, AuthorizedParty: instance.AuthorizedParty,
	}}
	raw, err := yaml.Marshal(document)
	if err != nil {
		return "", err
	}
	path := filepath.Join(resolved, "exports", instance.ManagedID+".static-instance.yaml")
	if err := atomicWritePrivate(path, raw); err != nil {
		return "", err
	}
	return path, nil
}

func prepareCAAndClient(prepared *PreparedControlIdentity) error {
	if rawCA, err := readSecureRegularFile(prepared.CAFile, 65536); err == nil {
		if _, _, err := loadCA(prepared.CAFile, prepared.CAKeyFile); err != nil {
			return err
		}
		if _, err := readSecureRegularFile(prepared.ClientCertFile, 65536); err != nil {
			return errors.New("incomplete existing control identity")
		}
		if _, err := readEd25519PrivateKey(prepared.ClientKeyFile); err != nil {
			return errors.New("incomplete existing control identity")
		}
		prepared.ClientCAPEM = string(rawCA)
		return nil
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	for _, path := range []string{prepared.CAKeyFile, prepared.ClientCertFile, prepared.ClientKeyFile} {
		if _, err := os.Lstat(path); err == nil {
			return errors.New("refusing to overwrite incomplete control identity")
		} else if !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	caPublic, caPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	serial, err := randomSerial()
	if err != nil {
		return err
	}
	now := time.Now
	if prepared.Options.Now != nil {
		now = prepared.Options.Now
	}
	caTemplate := &x509.Certificate{
		SerialNumber:          serial,
		Subject:               pkix.Name{CommonName: "Kessoku managed Starry " + prepared.Options.ManagedID},
		NotBefore:             now().UTC().Add(-5 * time.Minute),
		NotAfter:              now().UTC().Add(5 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
		BasicConstraintsValid: true,
		IsCA:                  true,
		MaxPathLenZero:        true,
	}
	caDER, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, caPublic, caPrivate)
	if err != nil {
		return err
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caDER})
	if err := writeEd25519PrivateKey(prepared.CAKeyFile, caPrivate); err != nil {
		return err
	}
	if err := atomicWritePrivate(prepared.CAFile, caPEM); err != nil {
		return err
	}
	clientPublic, clientPrivate, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return err
	}
	clientURI, err := url.Parse(prepared.AuthorizedParty)
	if err != nil {
		return err
	}
	serial, err = randomSerial()
	if err != nil {
		return err
	}
	clientTemplate := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Kessoku Starry control client " + prepared.Options.ManagedID},
		NotBefore:    now().UTC().Add(-5 * time.Minute),
		NotAfter:     now().UTC().Add(certificateLifetime),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		URIs:         []*url.URL{clientURI},
	}
	clientDER, err := x509.CreateCertificate(rand.Reader, clientTemplate, caTemplate, clientPublic, caPrivate)
	if err != nil {
		return err
	}
	if err := writeEd25519PrivateKey(prepared.ClientKeyFile, clientPrivate); err != nil {
		return err
	}
	if err := atomicWritePrivate(prepared.ClientCertFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: clientDER})); err != nil {
		return err
	}
	prepared.ClientCAPEM = string(caPEM)
	return nil
}

func prepareControlSigner(prepared *PreparedControlIdentity) error {
	privateKey, err := readEd25519PrivateKey(prepared.ControlKeyFile)
	if errors.Is(err, os.ErrNotExist) {
		_, privateKey, err = ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return err
		}
		if err := writeEd25519PrivateKey(prepared.ControlKeyFile, privateKey); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	publicKey := privateKey.Public().(ed25519.PublicKey)
	digest := sha256.Sum256(publicKey)
	prepared.ControlKeySHA256 = "sha256:" + hex.EncodeToString(digest[:])
	prepared.ControlKeyID = "kessoku-" + prepared.Options.ManagedID + "-" + hex.EncodeToString(digest[:8])
	prepared.ServiceJWKS = map[string]interface{}{
		"keys": []interface{}{map[string]interface{}{
			"kty": "OKP", "crv": "Ed25519", "use": "sig", "alg": "EdDSA",
			"kid": prepared.ControlKeyID, "x": base64.RawURLEncoding.EncodeToString(publicKey),
		}},
	}
	return nil
}

func loadCA(certificatePath, keyPath string) (*x509.Certificate, ed25519.PrivateKey, error) {
	raw, err := readSecureRegularFile(certificatePath, 65536)
	if err != nil {
		return nil, nil, err
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, nil, errors.New("invalid stored CA certificate")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil || !certificate.IsCA {
		return nil, nil, errors.New("invalid stored CA certificate")
	}
	key, err := readEd25519PrivateKey(keyPath)
	if err != nil {
		return nil, nil, err
	}
	certificateKey, ok := certificate.PublicKey.(ed25519.PublicKey)
	if !ok || !bytes.Equal(certificateKey, key.Public().(ed25519.PublicKey)) {
		return nil, nil, errors.New("stored CA key does not match certificate")
	}
	return certificate, key, nil
}

func parseAndVerifyCSR(value string) (*x509.CertificateRequest, error) {
	block, rest := pem.Decode([]byte(value))
	if block == nil || (block.Type != "CERTIFICATE REQUEST" && block.Type != "NEW CERTIFICATE REQUEST") || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, errors.New("invalid Agent CSR PEM")
	}
	csr, err := x509.ParseCertificateRequest(block.Bytes)
	if err != nil || csr.CheckSignature() != nil {
		return nil, errors.New("invalid Agent CSR signature")
	}
	return csr, nil
}

func csrContainsHost(csr *x509.CertificateRequest, host string) bool {
	if address := net.ParseIP(host); address != nil {
		for _, candidate := range csr.IPAddresses {
			if candidate.Equal(address) {
				return true
			}
		}
		return false
	}
	for _, candidate := range csr.DNSNames {
		if strings.EqualFold(candidate, host) {
			return true
		}
	}
	return false
}

func publicKeyFingerprint(publicKey crypto.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256(der)
	return "sha256:" + hex.EncodeToString(digest[:]), nil
}

func validateAgentCertificate(raw []byte, expected crypto.PublicKey, prepared PreparedControlIdentity) error {
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "CERTIFICATE" || len(strings.TrimSpace(string(rest))) != 0 {
		return errors.New("invalid Agent certificate")
	}
	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return err
	}
	expectedDER, err := x509.MarshalPKIXPublicKey(expected)
	if err != nil {
		return err
	}
	actualDER, err := x509.MarshalPKIXPublicKey(certificate.PublicKey)
	if err != nil || !bytes.Equal(actualDER, expectedDER) {
		return errors.New("Agent certificate public key mismatch")
	}
	ca, _, err := loadCA(prepared.CAFile, prepared.CAKeyFile)
	if err != nil {
		return err
	}
	if err := certificate.CheckSignatureFrom(ca); err != nil {
		return err
	}
	return certificate.VerifyHostname(prepared.Options.TLSServerName)
}

func readEd25519PrivateKey(path string) (ed25519.PrivateKey, error) {
	raw, err := readSecureRegularFile(path, 16384)
	if err != nil {
		return nil, err
	}
	block, rest := pem.Decode(raw)
	if block == nil || block.Type != "PRIVATE KEY" || len(strings.TrimSpace(string(rest))) != 0 {
		return nil, errors.New("invalid Ed25519 private key PEM")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, errors.New("invalid Ed25519 private key")
	}
	key, ok := parsed.(ed25519.PrivateKey)
	if !ok || len(key) != ed25519.PrivateKeySize {
		return nil, errors.New("private key is not Ed25519")
	}
	return key, nil
}

func writeEd25519PrivateKey(path string, key ed25519.PrivateKey) error {
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return atomicWritePrivate(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func readSecureRegularFile(path string, limit int64) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || !ownedByCurrentUser(info) || info.Size() > limit {
		return nil, ErrUnsafePermissions
	}
	return os.ReadFile(path)
}

func ensurePrivateDirectory(path string) error {
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		if err := os.Mkdir(path, 0o700); err != nil {
			return err
		}
		info, err = os.Lstat(path)
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o700 || !ownedByCurrentUser(info) {
		return ErrUnsafePermissions
	}
	return nil
}

func atomicWritePrivate(path string, contents []byte) error {
	directory := filepath.Dir(path)
	if err := ensurePrivateDirectory(directory); err != nil {
		return err
	}
	if info, err := os.Lstat(path); err == nil {
		if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 || info.Mode().Perm() != 0o600 || !ownedByCurrentUser(info) {
			return ErrUnsafePermissions
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	temporary, err := os.CreateTemp(directory, ".kessoku-write-")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(contents); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return err
	}
	return os.Chmod(path, 0o600)
}

func randomSerial() (*big.Int, error) {
	limit := new(big.Int).Lsh(big.NewInt(1), 128)
	return rand.Int(rand.Reader, limit)
}
