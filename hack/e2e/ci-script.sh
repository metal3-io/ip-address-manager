#!/usr/bin/env bash

# Copyright 2025 The Kubernetes Authors.
#
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
#
#     http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

# -----------------------------------------------------------------------------
# Description: This script sets up the environment and runs E2E tests for the
#              IPAM project.
# Usage:       From the root of the repo, run:
#              ./hack/e2e/ci-script.sh
# -----------------------------------------------------------------------------

set -o errexit
set -o nounset
set -o pipefail

REPO_ROOT=$(realpath "$(dirname "${BASH_SOURCE[0]}")/../..")
cd "${REPO_ROOT}" || exit 1

# Always collect artifacts before exiting
collect_artifacts() {
  if [[ -d "${REPO_ROOT}/test/e2e/_artifacts" ]]; then
    tar --directory "${REPO_ROOT}/test/e2e/_artifacts" -czf "artifacts-e2e-ipam.tar.gz" . 2>/dev/null || true
  fi
}
trap collect_artifacts EXIT

# CAPI test framework uses kubectl in the background
"${REPO_ROOT}/hack/e2e/ensure_kubectl.sh"
"${REPO_ROOT}/hack/e2e/ensure_go.sh"

# Verify they are available and have correct versions.
PATH=$PATH:/usr/local/go/bin
PATH=$PATH:$(go env GOPATH)/bin

# Build the IPAM image from the checked-out source.
# The image name must match what e2e_conf.yaml expects for Kind loading.
make docker-build-e2e

# When using an existing cluster, the CAPI framework skips image loading.
# Load the image into the kind cluster manually.
if [[ "${USE_EXISTING_CLUSTER:-false}" == "true" ]]; then
  kind load docker-image quay.io/metal3-io/ip-address-manager:e2e-test
fi

# Increase inotify limits to avoid "too many open files" errors during
# pivoting and clusterctl-upgrade e2e tests.
sudo sysctl fs.inotify.max_user_instances=1024
sudo sysctl fs.inotify.max_user_watches=524288

# Enable ClusterTopology feature for clusterctl-upgrade tests
export CLUSTER_TOPOLOGY=true

make test-e2e
