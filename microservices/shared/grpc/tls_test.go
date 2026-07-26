package grpc_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	sharedGRPC "github.com/bashocode/gowallet/microservices/shared/grpc"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestCertificates(t *testing.T) (dir, certPath, keyPath, caPath string) {
	tempDir, err := os.MkdirTemp("", "mtls-test-*")
	require.NoError(t, err)

	// 1. Generate CA
	caPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	caTemplate := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "gowallet-ca"},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		IsCA:                  true,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
	}

	caBytes, err := x509.CreateCertificate(rand.Reader, caTemplate, caTemplate, &caPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	caPath = filepath.Join(tempDir, "ca.crt")
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: caBytes})
	require.NoError(t, os.WriteFile(caPath, caPEM, 0644))

	// 2. Generate Server/Client Cert signed by CA
	certPrivKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	certTemplate := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "test-service"},
		NotBefore:    time.Now().Add(-1 * time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth, x509.ExtKeyUsageClientAuth},
		DNSNames:     []string{"test-service", "localhost"},
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, certTemplate, caTemplate, &certPrivKey.PublicKey, caPrivKey)
	require.NoError(t, err)

	certPath = filepath.Join(tempDir, "service.crt")
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certBytes})
	require.NoError(t, os.WriteFile(certPath, certPEM, 0644))

	keyPath = filepath.Join(tempDir, "service.key")
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(certPrivKey)})
	require.NoError(t, os.WriteFile(keyPath, keyPEM, 0600))

	return tempDir, certPath, keyPath, caPath
}

func TestLoadCredentials(t *testing.T) {
	tempDir, certPath, keyPath, caPath := generateTestCertificates(t)
	defer os.RemoveAll(tempDir)

	t.Run("LoadServerCredentials success", func(t *testing.T) {
		creds, err := sharedGRPC.LoadServerCredentials(certPath, keyPath, caPath)
		assert.NoError(t, err)
		assert.NotNil(t, creds)
		assert.Equal(t, "tls", creds.Info().SecurityProtocol)
	})

	t.Run("LoadClientCredentials success", func(t *testing.T) {
		creds, err := sharedGRPC.LoadClientCredentials(certPath, keyPath, caPath, "test-service")
		assert.NoError(t, err)
		assert.NotNil(t, creds)
		assert.Equal(t, "tls", creds.Info().SecurityProtocol)
	})

	t.Run("GetClientDialCredentials non-production fallback to insecure", func(t *testing.T) {
		creds, err := sharedGRPC.GetClientDialCredentials(false, "", "", "", "")
		assert.NoError(t, err)
		assert.NotNil(t, creds)
		assert.Equal(t, "insecure", creds.Info().SecurityProtocol)
	})

	t.Run("GetServerOptions non-production fallback to nil options", func(t *testing.T) {
		opts, err := sharedGRPC.GetServerOptions(false, "", "", "")
		assert.NoError(t, err)
		assert.Nil(t, opts)
	})
}
