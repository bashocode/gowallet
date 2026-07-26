package grpc

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"os"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// LoadServerCredentials loads TLS certificates and CA pool for a gRPC server requiring client mTLS authentication.
func LoadServerCredentials(certFile, keyFile, caFile string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load server TLS key pair: %w", err)
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate file '%s': %w", caFile, err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to append CA certificate to pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientCAs:    caPool,
		ClientAuth:   tls.RequireAndVerifyClientCert,
		MinVersion:   tls.VersionTLS13,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// LoadClientCredentials loads TLS certificates and CA pool for a gRPC client authenticating to an mTLS server.
func LoadClientCredentials(certFile, keyFile, caFile, serverNameOverride string) (credentials.TransportCredentials, error) {
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		return nil, fmt.Errorf("failed to load client TLS key pair: %w", err)
	}

	caCert, err := os.ReadFile(caFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read CA certificate file '%s': %w", caFile, err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(caCert) {
		return nil, errors.New("failed to append CA certificate to pool")
	}

	tlsConfig := &tls.Config{
		Certificates: []tls.Certificate{cert},
		RootCAs:      caPool,
		ServerName:   serverNameOverride,
		MinVersion:   tls.VersionTLS13,
	}

	return credentials.NewTLS(tlsConfig), nil
}

// GetClientDialCredentials returns transport credentials (mTLS or insecure) depending on configuration and environment.
func GetClientDialCredentials(isProduction bool, certFile, keyFile, caFile, serverNameOverride string) (credentials.TransportCredentials, error) {
	if !isProduction && (certFile == "" || keyFile == "" || caFile == "") {
		return insecure.NewCredentials(), nil
	}
	return LoadClientCredentials(certFile, keyFile, caFile, serverNameOverride)
}

// GetServerOptions returns server gRPC options including mTLS transport credentials if configured or in production profile.
func GetServerOptions(isProduction bool, certFile, keyFile, caFile string) ([]grpc.ServerOption, error) {
	if !isProduction && (certFile == "" || keyFile == "" || caFile == "") {
		return nil, nil
	}
	creds, err := LoadServerCredentials(certFile, keyFile, caFile)
	if err != nil {
		return nil, err
	}
	return []grpc.ServerOption{grpc.Creds(creds)}, nil
}
