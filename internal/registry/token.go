package registry

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base32"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/azayr/selfhost/internal/store"
	"github.com/golang-jwt/jwt/v5"
)

const tokenTTL = 15 * time.Minute

var ErrInvalidService = errors.New("registry token service does not match")

type TokenAuthConfig struct {
	Issuer          string
	Service         string
	PrivateKeyPath  string
	CertificatePath string
}

type TokenIssuer struct {
	config TokenAuthConfig
	key    *rsa.PrivateKey
	keyID  string
}

type TokenRequest struct {
	Service    string
	Scopes     []string
	Permission string
}

type AccessEntry struct {
	Type    string   `json:"type"`
	Name    string   `json:"name"`
	Actions []string `json:"actions"`
}

func NewTokenIssuer(config TokenAuthConfig) (*TokenIssuer, error) {
	if config.Issuer == "" || config.Service == "" {
		return nil, errors.New("registry token issuer and service are required")
	}
	if err := ensureTokenKeypair(config.PrivateKeyPath, config.CertificatePath); err != nil {
		return nil, err
	}
	key, err := readPrivateKey(config.PrivateKeyPath)
	if err != nil {
		return nil, err
	}
	keyID, err := libtrustKeyID(&key.PublicKey)
	if err != nil {
		return nil, err
	}
	return &TokenIssuer{config: config, key: key, keyID: keyID}, nil
}

func (i *TokenIssuer) Issue(user store.User, request TokenRequest) (string, int, time.Time, error) {
	service := strings.TrimSpace(request.Service)
	if service != "" && service != i.config.Service {
		return "", 0, time.Time{}, ErrInvalidService
	}
	return i.issue(user.Email, request.Service, allowedAccess(user.Role, request.Permission, request.Scopes))
}

// IssueServiceToken creates a token for Dokyr's own registry-management
// requests. These permissions never pass through the public token endpoint.
func (i *TokenIssuer) IssueServiceToken(access []AccessEntry) (string, error) {
	token, _, _, err := i.issue("dokyr-control-plane", "", access)
	return token, err
}

func (i *TokenIssuer) issue(subject, service string, access []AccessEntry) (string, int, time.Time, error) {
	now := time.Now().UTC()
	audience := strings.TrimSpace(service)
	if audience == "" {
		audience = i.config.Service
	}
	claims := jwt.MapClaims{
		"iss":    i.config.Issuer,
		"sub":    subject,
		"aud":    audience,
		"exp":    now.Add(tokenTTL).Unix(),
		"nbf":    now.Add(-30 * time.Second).Unix(),
		"iat":    now.Unix(),
		"jti":    randomTokenID(),
		"access": access,
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	token.Header["kid"] = i.keyID
	signed, err := token.SignedString(i.key)
	if err != nil {
		return "", 0, time.Time{}, err
	}
	return signed, int(tokenTTL.Seconds()), now, nil
}

func libtrustKeyID(publicKey *rsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(publicKey)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(der)
	encoded := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(sum[:30])
	groups := make([]string, 0, 12)
	for start := 0; start < len(encoded); start += 4 {
		groups = append(groups, encoded[start:start+4])
	}
	return strings.Join(groups, ":"), nil
}

func allowedAccess(role, permission string, scopes []string) []AccessEntry {
	canPush := permission == "read_write" && (role == "owner" || role == "admin" || role == "developer")
	entries := make([]AccessEntry, 0, len(scopes))
	for _, scope := range scopes {
		parts := strings.Split(scope, ":")
		if len(parts) != 3 || parts[0] != "repository" || strings.TrimSpace(parts[1]) == "" {
			continue
		}
		requested := map[string]bool{}
		for _, action := range strings.Split(parts[2], ",") {
			action = strings.TrimSpace(action)
			if action == "pull" || (action == "push" && canPush) {
				requested[action] = true
			}
		}
		actions := make([]string, 0, len(requested))
		for action := range requested {
			actions = append(actions, action)
		}
		sort.Strings(actions)
		if len(actions) == 0 {
			continue
		}
		entries = append(entries, AccessEntry{Type: parts[0], Name: parts[1], Actions: actions})
	}
	return entries
}

func ensureTokenKeypair(keyPath, certPath string) error {
	if _, keyErr := os.Stat(keyPath); keyErr == nil {
		if _, certErr := os.Stat(certPath); certErr == nil {
			return nil
		}
	}
	if err := os.MkdirAll(filepath.Dir(keyPath), 0o700); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(certPath), 0o755); err != nil {
		return err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	serialLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serial, err := rand.Int(rand.Reader, serialLimit)
	if err != nil {
		return err
	}
	template := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "Dokyr Registry Token Signing"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().AddDate(10, 0, 0),
		KeyUsage:     x509.KeyUsageDigitalSignature,
	}
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return err
	}
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	if err := os.WriteFile(keyPath, keyPEM, 0o600); err != nil {
		return err
	}
	return os.WriteFile(certPath, certPEM, 0o644)
}

func readPrivateKey(path string) (*rsa.PrivateKey, error) {
	body, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode(body)
	if block == nil {
		return nil, errors.New("registry token private key is not PEM")
	}
	return x509.ParsePKCS1PrivateKey(block.Bytes)
}

func randomTokenID() string {
	var value [16]byte
	if _, err := rand.Read(value[:]); err != nil {
		return ""
	}
	return base64.RawURLEncoding.EncodeToString(value[:])
}
