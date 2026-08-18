#!/usr/bin/env bash
# Generate a self-signed cert + key for the LAN-IP HTTPS endpoint.
# Usage: ./caddy/gen-cert.sh <lan-ip>
#
# Output: caddy/cert.pem, caddy/key.pem. Both are mounted into the Caddy
# container at /etc/caddy/. Caddy serves them via:
#   tls /etc/caddy/cert.pem /etc/caddy/key.pem
#
# For the mobile device to trust the cert, install cert.pem as a
# trusted profile on the phone (Settings → General → VPN & Device
# Management → Install profile on iOS; equivalent on Android).
set -euo pipefail

lan_ip="${1:?usage: $0 <lan-ip>}"

dir="$(cd "$(dirname "$0")" && pwd)"
cert="$dir/cert.pem"
key="$dir/key.pem"

openssl req -x509 -newkey rsa:4096 -nodes \
    -keyout "$key" \
    -out "$cert" \
    -days 365 \
    -subj "/CN=$lan_ip" \
    -addext "subjectAltName=IP:$lan_ip,DNS:localhost"

echo "wrote $cert"
echo "wrote $key"
echo
echo "Install $cert on each mobile device that will reach the app."
