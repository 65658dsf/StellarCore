package cert

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"errors"
	"fmt"
	"math"
	"math/big"
	"net"
	"time"
)

type SelfSignedCertGenerator struct {
	caKey  []byte
	caCert []byte
}

var _ Generator = &SelfSignedCertGenerator{}

// SetCA sets the PEM-encoded CA private key and CA cert for signing the generated serving cert.
func (cp *SelfSignedCertGenerator) SetCA(caKey, caCert []byte) {
	cp.caKey = caKey
	cp.caCert = caCert
}

// Generate creates and returns a CA certificate, certificate and
// key for the server or client. Key and Cert are used by the server or client
// to establish trust for others, CA certificate is used by the
// client or server to verify the other's authentication chain.
// The cert will be valid for 365 days.
func (cp *SelfSignedCertGenerator) Generate(commonName string) (*Artifacts, error) {
	var signingKey *rsa.PrivateKey
	var signingCert *x509.Certificate
	var valid bool
	var err error

	valid, signingKey, signingCert = cp.validCACert()
	if !valid {
		signingKey, err = NewPrivateKey()
		if err != nil {
			return nil, fmt.Errorf("failed to create the CA private key: %v", err)
		}
		signingCert, err = NewSelfSignedCACert(Config{CommonName: commonName}, signingKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create the CA cert: %v", err)
		}
	}

	hostIP := net.ParseIP(commonName)
	var altIPs []net.IP
	DNSNames := []string{"localhost"}
	if hostIP.To4() != nil {
		altIPs = append(altIPs, hostIP.To4())
	} else {
		DNSNames = append(DNSNames, commonName)
	}

	key, err := NewPrivateKey()
	if err != nil {
		return nil, fmt.Errorf("failed to create the private key: %v", err)
	}
	signedCert, err := NewSignedCert(
		Config{
			CommonName: commonName,
			Usages:     []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
			AltNames:   AltNames{IPs: altIPs, DNSNames: DNSNames},
		},
		key, signingCert, signingKey,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create the cert: %v", err)
	}
	return &Artifacts{
		Key:    EncodePrivateKeyPEM(key),
		Cert:   EncodeCertPEM(signedCert),
		CAKey:  EncodePrivateKeyPEM(signingKey),
		CACert: EncodeCertPEM(signingCert),
	}, nil
}

func (cp *SelfSignedCertGenerator) validCACert() (bool, *rsa.PrivateKey, *x509.Certificate) {
	if !ValidCACert(cp.caKey, cp.caCert, cp.caCert, "",
		time.Now().AddDate(1, 0, 0)) {
		return false, nil, nil
	}

	var ok bool
	key, err := ParsePrivateKeyPEM(cp.caKey)
	if err != nil {
		return false, nil, nil
	}
	privateKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return false, nil, nil
	}

	certs, err := ParseCertsPEM(cp.caCert)
	if err != nil {
		return false, nil, nil
	}
	if len(certs) != 1 {
		return false, nil, nil
	}
	return true, privateKey, certs[0]
}

type AltNames struct {
	DNSNames []string
	IPs      []net.IP
}

type Config struct {
	CommonName   string
	Organization []string
	Usages       []x509.ExtKeyUsage
	AltNames     AltNames
}

// NewPrivateKey creates an RSA private key
func NewPrivateKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

// NewSignedCert creates a signed certificate using the given CA certificate and key
func NewSignedCert(cfg Config, key crypto.Signer, caCert *x509.Certificate, caKey crypto.Signer) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if len(cfg.CommonName) == 0 {
		return nil, errors.New("must specify a CommonName")
	}
	if len(cfg.Usages) == 0 {
		return nil, errors.New("must specify at least one ExtKeyUsage")
	}

	certTmpl := x509.Certificate{
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		DNSNames:     cfg.AltNames.DNSNames,
		IPAddresses:  cfg.AltNames.IPs,
		SerialNumber: serial,
		NotBefore:    caCert.NotBefore,
		NotAfter:     time.Now().Add(time.Hour * 24 * 365 * 10).UTC(),
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  cfg.Usages,
	}
	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, caCert, key.Public(), caKey)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

func NewSelfSignedCACert(cfg Config, key *rsa.PrivateKey) (*x509.Certificate, error) {
	serial, err := rand.Int(rand.Reader, new(big.Int).SetInt64(math.MaxInt64))
	if err != nil {
		return nil, err
	}
	if cfg.CommonName == "" {
		return nil, errors.New("must specify a CommonName")
	}

	now := time.Now().UTC()
	certTmpl := x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName:   cfg.CommonName,
			Organization: cfg.Organization,
		},
		NotBefore:             now.Add(-10 * time.Minute),
		NotAfter:              now.Add(10 * 365 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certDERBytes, err := x509.CreateCertificate(rand.Reader, &certTmpl, &certTmpl, key.Public(), key)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(certDERBytes)
}

func ParseCertsPEM(pemCerts []byte) ([]*x509.Certificate, error) {
	var certs []*x509.Certificate
	for {
		block, rest := pem.Decode(pemCerts)
		if block == nil {
			break
		}
		pemCerts = rest
		if block.Type != certificateBlockType {
			continue
		}
		c, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	if len(certs) == 0 {
		return nil, errors.New("no certificates found")
	}
	return certs, nil
}

func ParsePrivateKeyPEM(pemKey []byte) (any, error) {
	block, _ := pem.Decode(pemKey)
	if block == nil {
		return nil, errors.New("failed to decode PEM private key")
	}

	if pk, err := x509.ParsePKCS1PrivateKey(block.Bytes); err == nil {
		return pk, nil
	}
	pk, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	return pk, nil
}

const (
	rsaPrivateKeyBlockType = "RSA PRIVATE KEY"
	certificateBlockType   = "CERTIFICATE"
)

// EncodePrivateKeyPEM returns PEM-encoded private key data
func EncodePrivateKeyPEM(key *rsa.PrivateKey) []byte {
	block := pem.Block{
		Type:  rsaPrivateKeyBlockType,
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}
	return pem.EncodeToMemory(&block)
}

// EncodeCertPEM returns PEM-encoded certificate data
func EncodeCertPEM(ct *x509.Certificate) []byte {
	block := pem.Block{
		Type:  certificateBlockType,
		Bytes: ct.Raw,
	}
	return pem.EncodeToMemory(&block)
}
