#!/bin/bash

# Create a self-signed certificate for the relayderp server
# that will be onbaorded by integration tests

NAMESPACE="nexodus"
PUBLIC_DERP_CERT_NAME="nexodus-derp-relay-cert"
ONBOARD_DERP_CERT_NAME="nexodus-onboard-derp-relay-cert"

# Check if the Certificate already exists
if kubectl get certificate -n $NAMESPACE $PUBLIC_DERP_CERT_NAME > /dev/null 2>&1; then
    echo "Certificate $PUBLIC_DERP_CERT_NAME already exists. Skipping creation."
else
    echo "Certificate $PUBLIC_DERP_CERT_NAME does not exist. Creating..."

    # Your manifest content with namespace
cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: $PUBLIC_DERP_CERT_NAME
  namespace: $NAMESPACE
spec:
  secretName: $PUBLIC_DERP_CERT_NAME
  duration: 2160h0m0s
  renewBefore: 360h0m0s
  subject:
    organizations:
      - nexodus
  privateKey:
    algorithm: RSA
    encoding: PKCS1
    size: 2048
  usages:
    - server auth
    - client auth
  dnsNames:
    - relay.nexodus.io
  issuerRef:
    name: nexodus-issuer
    kind: Issuer
EOF
fi

# Wait for the certificate to be ready
kubectl wait -n nexodus --for=condition=Ready certificate/nexodus-derp-relay-cert

# Check if the Certificate already exists
if kubectl get certificate -n $NAMESPACE $ONBOARD_DERP_CERT_NAME > /dev/null 2>&1; then
    echo "Certificate $ONBOARD_DERP_CERT_NAME already exists. Skipping creation."
else
    echo "Certificate $ONBOARD_DERP_CERT_NAME does not exist. Creating..."

    # Your manifest content with namespace
cat <<EOF | kubectl apply -n $NAMESPACE -f -
apiVersion: cert-manager.io/v1
kind: Certificate
metadata:
  name: $ONBOARD_DERP_CERT_NAME
  namespace: $NAMESPACE
spec:
  secretName: $ONBOARD_DERP_CERT_NAME
  duration: 2160h0m0s
  renewBefore: 360h0m0s
  subject:
    organizations:
      - nexodus
  privateKey:
    algorithm: RSA
    encoding: PKCS1
    size: 2048
  usages:
    - server auth
    - client auth
  dnsNames:
    - custom.relay.nexodus.io
  issuerRef:
    name: nexodus-issuer
    kind: Issuer
EOF
fi

# Wait for the certificate to be ready
kubectl wait -n nexodus --for=condition=Ready certificate/nexodus-onboard-derp-relay-cert