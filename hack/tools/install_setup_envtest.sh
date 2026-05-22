#!/usr/bin/env bash

set -o errexit
set -o nounset
set -o pipefail

[[ -f bin/setup-envtest ]] && exit 0

if ! command -v sha256sum &>/dev/null; then
    echo "ERROR: sha256sum not found. On macOS, install coreutils: brew install coreutils" >&2
    exit 1
fi

version=v0.24.1
arch=$(go env GOARCH)
os=$(go env GOOS)

# SHA256 checksums from https://github.com/kubernetes-sigs/controller-runtime/releases/tag/${version}
key="${os}-${arch}"
case "${key}" in
    darwin-amd64)  SETUP_ENVTEST_SHA256="3fb17f2b1b0f09b7e5395180bd2bcb1d53bb78d72bb0415106b7ae8bf64e23d0" ;;
    darwin-arm64) SETUP_ENVTEST_SHA256="7e59a0d526f6946aa2f114d34b2e309639c811f3a4f83d56f37b6e3197c6fdfb" ;;
    linux-amd64)  SETUP_ENVTEST_SHA256="a9a78fadfc338a38188332f36863c76877f1c86df1a83d2241d2bfc3935297d2" ;;
    linux-arm64) SETUP_ENVTEST_SHA256="c5d8968ec3f2a120b66bc13bd36f80fe4150c34aae7cc491bf9624c8680296c7" ;;
    *)
        echo "ERROR: No pinned SHA256 for ${key}. Add it to this script." >&2
        exit 1
        ;;
esac

mkdir -p ./bin

url="https://github.com/kubernetes-sigs/controller-runtime/releases/download/${version}/setup-envtest-${os}-${arch}"
curl -sfL "${url}" -o bin/setup-envtest

actual_sha=$(sha256sum bin/setup-envtest | awk '{print $1}')
if [[ "${actual_sha}" != "${SETUP_ENVTEST_SHA256}" ]]; then
    echo "ERROR: SHA256 mismatch for setup-envtest ${key}" >&2
    echo "  expected: ${SETUP_ENVTEST_SHA256}" >&2
    echo "  actual:   ${actual_sha}" >&2
    rm -f bin/setup-envtest
    exit 1
fi

chmod +x bin/setup-envtest
