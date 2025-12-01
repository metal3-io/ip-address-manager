#!/usr/bin/env bash

# -----------------------------------------------------------------------------
# Description: This script sets up the environment and runs E2E tests for the
#              IPAM project. It uses ushy-tools as bmc protocol emulator.
#              Supported protocols are: redfish and redfish-virtualmedia.
# Usage:       From the root of the repo, run:
#              ./test/e2e/ci-e2e.sh
# -----------------------------------------------------------------------------

set -eux

REPO_ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/../..")
cd "${REPO_ROOT}" || exit 1

export REPOS_DIR="${REPOS_DIR:-${REPO_ROOT}/test/e2e/_github.com}"
mkdir -p "${REPOS_DIR}"
BMOPATH="${REPOS_DIR}/baremetal-operator"
CAPM3PATH="${REPOS_DIR}/cluster-api-provider-metal3"

# Set environment variables for the e2e tests
export IPAMPATH="${REPO_ROOT}"
FORCE_REPO_UPDATE="${FORCE_REPO_UPDATE:-false}"

export CAPM3RELEASEBRANCH="${CAPM3RELEASEBRANCH:-main}"
export IPAMRELEASEBRANCH="${IPAMRELEASEBRANCH:-main}"

# Extract release version from release-branch name
if [[ "${CAPM3RELEASEBRANCH}" == release-* ]]; then
  CAPM3_RELEASE_PREFIX="${CAPM3RELEASEBRANCH#release-}"
  export CAPM3RELEASE="v${CAPM3_RELEASE_PREFIX}.99"
  export IPAMRELEASE="v${CAPM3_RELEASE_PREFIX}.99"
  export CAPI_RELEASE_PREFIX="v${CAPM3_RELEASE_PREFIX}."
else
  export CAPM3RELEASE="v1.12.99"
  export IPAMRELEASE="v1.12.99"
  export CAPI_RELEASE_PREFIX="v1.11."
fi

# Default CAPI_CONFIG_FOLDER to $HOME/.config folder if XDG_CONFIG_HOME not set
CONFIG_FOLDER="${XDG_CONFIG_HOME:-$HOME/.config}"
export CAPI_CONFIG_FOLDER="${CONFIG_FOLDER}/cluster-api"

# shellcheck source=./test/e2e/scripts/environment.sh
source "${REPO_ROOT}/hack/e2e/environment.sh"

clone_repo "https://github.com/metal3-io/baremetal-operator.git" "main" "${BMOPATH}"
clone_repo "https://github.com/metal3-io/cluster-api-provider-metal3.git" "main" "${CAPM3PATH}"

# Ensure requirements are installed
"${BMOPATH}/hack/e2e/ensure_go.sh"
# CAPI test framework uses kubectl in the background
"${BMOPATH}/hack/e2e/ensure_kubectl.sh"
"${BMOPATH}/hack/e2e/ensure_yq.sh"

# Verify they are available and have correct versions.
PATH=$PATH:/usr/local/go/bin
PATH=$PATH:$(go env GOPATH)/bin

make kustomize

sudo apt-get update
sudo apt-get install -y libvirt-dev pkg-config

# Clone BMO repo and install vbmctl and for other helpful scripts
if ! command -v vbmctl >/dev/null 2>&1; then
  pushd "${BMOPATH}/test/vbmctl"
  go build -tags=e2e,integration -o vbmctl ./main.go
  sudo install vbmctl /usr/local/bin/vbmctl
  popd
fi

case "${GINKGO_FOCUS:-}" in
  *upgrade*)
    mkdir -p "${CAPI_CONFIG_FOLDER}"
    echo "ENABLE_BMH_NAME_BASED_PREALLOCATION: true" >"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
    ;;
  *)
    export GINKGO_SKIP="upgrade"
    mkdir -p "${CAPI_CONFIG_FOLDER}"
    echo "ENABLE_BMH_NAME_BASED_PREALLOCATION: true" >"${CAPI_CONFIG_FOLDER}/clusterctl.yaml"
  ;;
esac

VIRSH_NETWORKS=("baremetal-e2e")
for network in "${VIRSH_NETWORKS[@]}"; do
  if ! sudo virsh net-list --all | grep "${network}"; then
    virsh -c qemu:///system net-define "${REPO_ROOT}/hack/e2e/${network}.xml"
    virsh -c qemu:///system net-start "${network}"
    virsh -c qemu:///system net-autostart "${network}"
  fi
done

# We need to create veth pair to connect metal3 net (defined above) and kind
# docker subnet. Let us start by creating a docker network with pre-defined
# name for bridge, so that we can configure the veth pair correctly.
# Also assume that if kind net exists, it is created by us.
if ! docker network list | grep kind; then
    # These options are used by kind itself. It uses docker default mtu and
    # generates ipv6 subnet ULA, but we can fix the ULA. Only addition to kind
    # options is the network bridge name.
    docker network create -d=bridge \
        -o com.docker.network.bridge.enable_ip_masquerade=true \
        -o com.docker.network.driver.mtu=1500 \
        -o com.docker.network.bridge.name="kind-bridge" \
        --ipv6 --subnet "fc00:f853:ccd:e793::/64" \
        kind
fi
docker network list

# Next create the veth pair
if ! ip a | grep metalend; then
    sudo ip link add metalend type veth peer name kindend
    sudo ip link set metalend master metal3
    sudo ip link set kindend master kind-bridge
    sudo ip link set metalend up
    sudo ip link set kindend up
fi
ip a

# Then we need to set routing rules as well
if ! sudo iptables -L FORWARD -v -n | grep kind-bridge; then
    sudo iptables -I FORWARD -i kind-bridge -o metal3 -j ACCEPT
    sudo iptables -I FORWARD -i metal3 -o kind-bridge -j ACCEPT
fi
sudo iptables -L FORWARD -n -v

# This IP is defined by the network we created above. It is sushy-tools / image
# server endpoint, not ironic.
IP_ADDRESS="192.168.222.1"

# E2E_BMCS_CONF_FILE="${REPO_ROOT}/test/e2e/config/bmcs.yaml"
export E2E_BMCS_CONF_FILE="${REPO_ROOT}/test/e2e/config/bmcs-redfish-virtualmedia.yaml"
vbmctl --yaml-source-file "${E2E_BMCS_CONF_FILE}"

# Sushy-tools variables
SUSHY_EMULATOR_FILE="${REPO_ROOT}"/test/e2e/data/sushy-tools/sushy-emulator.conf
# Start sushy-tools
docker start sushy-tools || docker run --name sushy-tools -d --network host \
  -v "${SUSHY_EMULATOR_FILE}":/etc/sushy/sushy-emulator.conf:Z \
  -v /var/run/libvirt:/var/run/libvirt:Z \
  -e SUSHY_EMULATOR_CONFIG=/etc/sushy/sushy-emulator.conf \
  quay.io/metal3-io/sushy-tools:latest sushy-emulator

# Image server variables
CIRROS_VERSION="0.6.2"
IMAGE_FILE="cirros-${CIRROS_VERSION}-x86_64-disk.img"
export IMAGE_CHECKSUM="c8fc807773e5354afe61636071771906"
export IMAGE_URL="http://${IP_ADDRESS}/${IMAGE_FILE}"
IMAGE_DIR="${REPO_ROOT}/test/e2e/images"
mkdir -p "${IMAGE_DIR}"

## Download disk images
if [[ ! -f "${IMAGE_DIR}/${IMAGE_FILE}" ]]; then
  wget --quiet -P "${IMAGE_DIR}/" https://artifactory.nordix.org/artifactory/metal3/images/iso/"${IMAGE_FILE}"
  wget --quiet -P "${IMAGE_DIR}/" https://artifactory.nordix.org/artifactory/metal3/images/sysrescue/systemrescue-11.00-amd64.iso
fi

## Download IPA (Ironic Python Agent) image
# Ironic IPA downloader is configured to use this local image in the tests.
# This saves time, especially during ironic upgrade tests and also
# gives us early failure in case there is some issue downloading it.
IPA_FILE="ipa-centos9-master.tar.gz"
IPA_BASEURI=https://artifactory.nordix.org/artifactory/openstack-remote-cache/ironic-python-agent/dib/
if [[ ! -f "${IMAGE_DIR}/${IPA_FILE}" ]]; then
  wget --quiet -P "${IMAGE_DIR}/" "${IPA_BASEURI}/${IPA_FILE}"
fi

## Start the image server
docker start image-server-e2e || docker run --name image-server-e2e -d \
  -p 80:8080 \
  -v "${IMAGE_DIR}:/usr/share/nginx/html" nginxinc/nginx-unprivileged

# Generate ssh key pair for verifying provisioned BMHs
if [[ ! -f "${IMAGE_DIR}/ssh_testkey" ]]; then
  ssh-keygen -t ed25519 -f "${IMAGE_DIR}/ssh_testkey" -q -N ""
fi
pub_ssh_key=$(cut -d " " -f "1,2" "${IMAGE_DIR}/ssh_testkey.pub")

# Build an ISO image with baked ssh key
# See https://www.system-rescue.org/scripts/sysrescue-customize/
# We use the systemrescue ISO and their script for customizing it.
if [[ ! -f "${IMAGE_DIR}/sysrescue-out.iso" ]];then
  pushd "${IMAGE_DIR}"
  wget -O sysrescue-customize https://gitlab.com/systemrescue/systemrescue-sources/-/raw/main/airootfs/usr/share/sysrescue/bin/sysrescue-customize?inline=false
  chmod +x sysrescue-customize

  mkdir -p recipe/iso_add/sysrescue.d
  # Reference: https://www.system-rescue.org/manual/Configuring_SystemRescue/
  cat << EOF > recipe/iso_add/sysrescue.d/90-config.yaml
---
global:
    nofirewall: true
sysconfig:
    authorized_keys:
        "test@example.com": "${pub_ssh_key}"
EOF

  ./sysrescue-customize --auto --recipe-dir recipe --source systemrescue-11.00-amd64.iso --dest=sysrescue-out.iso
  popd
fi
export ISO_IMAGE_URL="http://${IP_ADDRESS}/sysrescue-out.iso"

# Generate credentials
BMO_OVERLAYS=(
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.9"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.10"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-0.11"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/pr-test"
  "${REPO_ROOT}/test/e2e/data/bmo-deployment/overlays/release-latest"
)

IRONIC_USERNAME="$(uuidgen)"
IRONIC_PASSWORD="$(uuidgen)"

# These must be exported so that envsubst can pick them up below
export IRONIC_USERNAME
export IRONIC_PASSWORD

for overlay in "${BMO_OVERLAYS[@]}"; do
  echo "${IRONIC_USERNAME}" > "${overlay}/ironic-username"
  echo "${IRONIC_PASSWORD}" > "${overlay}/ironic-password"
done

IRSO_IRONIC_AUTH_DIR="${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/components/basic-auth"
echo "${IRONIC_USERNAME}" > "${IRSO_IRONIC_AUTH_DIR}/ironic-username"
echo "${IRONIC_PASSWORD}" > "${IRSO_IRONIC_AUTH_DIR}/ironic-password"

sed -i "s|SSH_PUB_KEY_CONTENT|${pub_ssh_key}|" "${REPO_ROOT}/test/e2e/data/ironic-standalone-operator/ironic/base/ironic.yaml"
# We need to gather artifacts/logs before exiting also if there are errors
set +e

# Run the e2e tests
make test-e2e
test_status="$?"

LOGS_DIR="${REPO_ROOT}/test/e2e/_artifacts/logs"
mkdir -p "${LOGS_DIR}/qemu"
sudo sh -c "cp -r /var/log/libvirt/qemu/* ${LOGS_DIR}/qemu/" || true
sudo chown -R "${USER}:${USER}" "${LOGS_DIR}/qemu"

# Collect all artifacts
tar --directory ${REPO_ROOT}/test/e2e/_artifacts/ -czf "artifacts-e2e-ipam.tar.gz" .

exit "${test_status}"
