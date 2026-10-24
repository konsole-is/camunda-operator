#!/usr/bin/env bash
# Writes a private CA and a server certificate that it signs, for the Git
# server of the kind harness of the fleet manager.
#
# Usage: tls.sh <out-dir>
#
# It writes ca.key, ca.crt, tls.key, and tls.crt into <out-dir>. The server
# certificate is valid for the Service git in the namespace fleet-harness,
# and for localhost, so a port-forward can use it too.
set -euo pipefail

out=${1:?usage: tls.sh <out-dir>}
mkdir -p "$out"

openssl req -x509 -newkey rsa:2048 -nodes -days 2 \
  -subj "/CN=fleet-harness-ca" \
  -keyout "$out/ca.key" -out "$out/ca.crt" 2>/dev/null

openssl req -newkey rsa:2048 -nodes \
  -subj "/CN=git.fleet-harness.svc" \
  -keyout "$out/tls.key" -out "$out/tls.csr" 2>/dev/null

printf '%s\n' \
  "subjectAltName=DNS:git.fleet-harness.svc,DNS:git.fleet-harness.svc.cluster.local,DNS:localhost,IP:127.0.0.1" \
  "basicConstraints=CA:FALSE" \
  "keyUsage=digitalSignature,keyEncipherment" \
  "extendedKeyUsage=serverAuth" >"$out/tls.ext"

openssl x509 -req -days 2 -in "$out/tls.csr" \
  -CA "$out/ca.crt" -CAkey "$out/ca.key" -CAcreateserial \
  -extfile "$out/tls.ext" -out "$out/tls.crt" 2>/dev/null

rm -f "$out/tls.csr" "$out/tls.ext" "$out/ca.srl"
