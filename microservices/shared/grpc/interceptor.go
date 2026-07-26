package grpc

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/peer"
	"google.golang.org/grpc/status"
)

const ServiceIdentityMetadataKey = "x-service-identity"

// RequireServiceIdentity creates a gRPC server unary interceptor that enforces workload identity allowlists.
// In production mode (allowInsecureFallback = false), mTLS peer certificates are strictly required.
func RequireServiceIdentity(allowInsecureFallback bool, allowed ...string) grpc.UnaryServerInterceptor {
	allowedSet := make(map[string]struct{}, len(allowed))
	for _, identity := range allowed {
		allowedSet[identity] = struct{}{}
	}

	return func(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
		identity, err := CallerIdentityFromContextWithFallback(ctx, allowInsecureFallback)
		if err != nil {
			return nil, status.Error(codes.Unauthenticated, "service identity required")
		}
		if len(allowedSet) > 0 {
			if _, ok := allowedSet[identity]; !ok {
				return nil, status.Error(codes.PermissionDenied, fmt.Sprintf("caller '%s' is not allowed to access %s", identity, info.FullMethod))
			}
		}
		return handler(ctx, req)
	}
}

// CallerIdentityFromContext extracts the calling service identity from TLS peer certificates.
func CallerIdentityFromContext(ctx context.Context) (string, error) {
	return CallerIdentityFromContextWithFallback(ctx, false)
}

// CallerIdentityFromContextWithFallback extracts the calling service identity from TLS peer certificates or context metadata.
func CallerIdentityFromContextWithFallback(ctx context.Context, allowInsecureFallback bool) (string, error) {
	// 1. Inspect mTLS Peer certificate if present
	if p, ok := peer.FromContext(ctx); ok && p.AuthInfo != nil {
		if tlsInfo, ok := p.AuthInfo.(credentials.TLSInfo); ok {
			if len(tlsInfo.State.PeerCertificates) > 0 {
				cert := tlsInfo.State.PeerCertificates[0]
				if cert.Subject.CommonName != "" {
					return cert.Subject.CommonName, nil
				}
				if len(cert.DNSNames) > 0 {
					return cert.DNSNames[0], nil
				}
				if len(cert.URIs) > 0 {
					return cert.URIs[0].String(), nil
				}
			}
		}
	}

	// 2. Fall back to gRPC metadata header (e.g. x-service-identity) only in non-production/test profiles
	if allowInsecureFallback {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			if val := md.Get(ServiceIdentityMetadataKey); len(val) > 0 && val[0] != "" {
				return val[0], nil
			}
		}
	}

	return "", errors.New("service identity required via mTLS peer certificate")
}

// UnaryClientIdentity attaches client service identity metadata to all outgoing gRPC calls.
func UnaryClientIdentity(clientServiceName string) grpc.UnaryClientInterceptor {
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		ctx = metadata.AppendToOutgoingContext(ctx, ServiceIdentityMetadataKey, clientServiceName)
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// UnaryClientTimeout ensures every gRPC client call carries a deadline.
func UnaryClientTimeout(defaultTimeout time.Duration) grpc.UnaryClientInterceptor {
	if defaultTimeout <= 0 {
		defaultTimeout = 5 * time.Second
	}
	return func(ctx context.Context, method string, req, reply any, cc *grpc.ClientConn, invoker grpc.UnaryInvoker, opts ...grpc.CallOption) error {
		if _, ok := ctx.Deadline(); !ok {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, defaultTimeout)
			defer cancel()
		}
		return invoker(ctx, method, req, reply, cc, opts...)
	}
}

// ValidateCertificates checks client/server TLS certificate validity.
func ValidateCertificates(cert *x509.Certificate) error {
	if cert == nil {
		return errors.New("nil certificate")
	}
	now := time.Now()
	if now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return errors.New("certificate is expired or not yet valid")
	}
	return nil
}
