#!/usr/bin/env bash

set -eux

[[ -f bin/kustomize ]] && exit 0

if ! command -v sha256sum &>/dev/null; then
    echo "ERROR: sha256sum not found. On macOS, install coreutils: brew install coreutils" >&2
    exit 1
fi

version=v5.8.1
arch=$(go env GOARCH)
os=$(go env GOOS)

mkdir -p ./bin

CHECKSUMS_URL="https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${version}/checksums.txt"
CHECKSUMS_FILE="checksums.txt"
BINARY_FILE="kustomize_${version}_${os}_${arch}.tar.gz"
URL="https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2F${version}/${BINARY_FILE}"

# Download checksums file
curl --proto '=https' --tlsv1.3 -sSfL \
    --retry 3 --retry-delay 5 --max-time 120 \
    -o "${CHECKSUMS_FILE}" "${CHECKSUMS_URL}"

# Extract the checksum for this platform
KUSTOMIZE_SHA256="$(grep "kustomize_${version}_${os}_${arch}\.tar\.gz$" "${CHECKSUMS_FILE}" | awk '{print $1;}')"
if [[ -z "${KUSTOMIZE_SHA256}" ]]; then
    echo >&2 "fatal: could not find checksum for ${BINARY_FILE} in ${CHECKSUMS_URL}"
    rm -f "${CHECKSUMS_FILE}"
    exit 1
fi

# Download binary
curl --proto '=https' --tlsv1.3 -sSfL \
    --retry 3 --retry-delay 5 --max-time 120 \
    -o "${BINARY_FILE}" "${URL}"

# Verify checksum before extraction
checksum="$(sha256sum "${BINARY_FILE}" | awk '{print $1;}')"
if [[ "${checksum}" != "${KUSTOMIZE_SHA256}" ]]; then
    echo >&2 "fatal: ${URL} checksum '${checksum}' differs from expected '${KUSTOMIZE_SHA256}'"
    rm -f "${BINARY_FILE}" "${CHECKSUMS_FILE}"
    exit 1
fi

# Extract
tar -xzvf "${BINARY_FILE}" -C ./bin

chmod 0755 ./bin/kustomize

rm -f "${BINARY_FILE}" "${CHECKSUMS_FILE}"
