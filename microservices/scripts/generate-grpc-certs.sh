#!/usr/bin/env bash
set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CERTS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)/certs"

mkdir -p "${CERTS_DIR}"
cd "${CERTS_DIR}"

echo "Generating Root CA..."
if [ ! -f "ca.key" ]; then
    openssl genrsa -out ca.key 4096
    openssl req -x509 -new -nodes -key ca.key -sha256 -days 3650 -out ca.crt -subj "/CN=gowallet-root-ca"
    echo "Root CA generated: ca.crt, ca.key"
else
    echo "Root CA already exists."
fi

SERVICES=(
    "auth-service"
    "user-service"
    "wallet-service"
    "ledger-service"
    "transaction-service"
    "payment-service"
    "scheduler-service"
    "notification-service"
    "api-gateway"
)

for SERVICE in "${SERVICES[@]}"; do
    echo "Generating unique mTLS certificate for ${SERVICE}..."

    CONFIG_FILE="${SERVICE}.cnf"
    cat > "${CONFIG_FILE}" <<EOF
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = ${SERVICE}

[v3_req]
basicConstraints = CA:FALSE
keyUsage = digitalSignature, keyEncipherment
extendedKeyUsage = serverAuth, clientAuth
subjectAltName = @alt_names

[alt_names]
DNS.1 = ${SERVICE}
DNS.2 = localhost
IP.1 = 127.0.0.1
EOF

    openssl genrsa -out "${SERVICE}.key" 2048
    openssl req -new -key "${SERVICE}.key" -out "${SERVICE}.csr" -config "${CONFIG_FILE}"
    openssl x509 -req -in "${SERVICE}.csr" -CA ca.crt -CAkey ca.key -CAcreateserial -out "${SERVICE}.crt" -days 365 -sha256 -extfile "${CONFIG_FILE}" -extensions v3_req

    rm -f "${SERVICE}.csr" "${CONFIG_FILE}"
    chmod 600 "${SERVICE}.key"
    chmod 644 "${SERVICE}.crt"
    echo "Certificate generated for ${SERVICE}"
done

chmod 600 ca.key
chmod 644 ca.crt

echo "All gRPC mTLS certificates generated successfully in ${CERTS_DIR}"
