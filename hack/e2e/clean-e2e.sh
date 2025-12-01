#!/usr/bin/env bash

REPO_ROOT=$(dirname "${BASH_SOURCE[0]}")/..
cd "${REPO_ROOT}" || exit 1

docker rm -f image-server-e2e
docker rm -f sushy-tools

BMH_NAME_REGEX="${1:-^bmh-test-}"
# Get a list of all virtual machines
VM_LIST=$(virsh -c qemu:///system list --all --name | grep "${BMH_NAME_REGEX}")

if [[ -n "${VM_LIST}" ]]; then
    # Loop through the list and delete each virtual machine
    for vm_name in ${VM_LIST}; do
        virsh -c qemu:///system destroy --domain "${vm_name}"
        virsh -c qemu:///system undefine --domain "${vm_name}" --remove-all-storage --nvram
        kubectl delete baremetalhost "${vm_name}"
    done
else
    echo "No virtual machines found. Skipping..."
fi

# Cleanup VM and volume qcow2
rm -rf /tmp/bmo-e2e-*.qcow2
rm -rf /tmp/pool_oo/bmo-e2e-*.qcow2


rm -rf "${REPO_ROOT}/test/e2e/_artifacts"
rm -rf "${REPO_ROOT}"/artifacts-*
rm -rf "${REPO_ROOT}/test/e2e/images"
rm -rf "${REPO_ROOT}/test/e2e/_github.com"

# Clear network
virsh -c qemu:///system net-destroy baremetal-e2e
virsh -c qemu:///system net-undefine baremetal-e2e

virsh -c qemu:///system net-destroy provisioning
virsh -c qemu:///system net-undefine provisioning

# Clean volume pool
virsh -c qemu:///system pool-destroy baremetal-e2e || true
virsh -c qemu:///system pool-delete baremetal-e2e || true
virsh -c qemu:///system pool-undefine baremetal-e2e || true
