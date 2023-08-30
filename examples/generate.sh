#!/bin/bash
# Copyright 2019 The Kubernetes Authors.
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

set -o errexit
set -o nounset
set -o pipefail

# Directories.
SOURCE_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" >/dev/null && pwd )"
OUTPUT_DIR=${OUTPUT_DIR:-${SOURCE_DIR}/_out}

# Cluster.
export CLUSTER_NAME="${CLUSTER_NAME:-test1}"
export NAMESPACE="${NAMESPACE:-capm3-system}"

# Outputs.
COMPONENTS_CERT_MANAGER_GENERATED_FILE=${OUTPUT_DIR}/cert-manager.yaml
COMPONENTS_CLUSTER_API_GENERATED_FILE=${SOURCE_DIR}/provider-components/core-components.yaml
COMPONENTS_METAL3_GENERATED_FILE=${SOURCE_DIR}/provider-components/infrastructure-components.yaml

PROVIDER_COMPONENTS_GENERATED_FILE=${OUTPUT_DIR}/provider-components.yaml
IPPOOL_GENERATED_FILE=${OUTPUT_DIR}/ippool.yaml

# Overwrite flag.
OVERWRITE=0

SCRIPT=$(basename "$0")
while test $# -gt 0; do
        case "$1" in
          -h|--help)
            echo "$SCRIPT - generates input yaml files for Cluster API on metal3"
            echo " "
            echo "$SCRIPT [options]"
            echo " "
            echo "options:"
            echo "-h, --help                show brief help"
            echo "-f, --force-overwrite     if file to be generated already exists, force script to overwrite it"
            exit 0
            ;;
          -f)
            OVERWRITE=1
            shift
            ;;
          --force-overwrite)
            OVERWRITE=1
            shift
            ;;
          *)
            break
            ;;
        esac
done

if [ $OVERWRITE -ne 1 ] && [ -d "$OUTPUT_DIR" ]; then
  echo "ERR: Folder ${OUTPUT_DIR} already exists. Delete it manually before running this script."
  exit 1
fi

mkdir -p "${OUTPUT_DIR}"
kubectl create namespace "${NAMESPACE}"

# Get enhanced envsubst version to evaluate expressions like ${VAR:=default}
ENVSUBST="${OUTPUT_DIR}/envsubst-go"
curl --fail -Ss -L -o "${ENVSUBST}" https://github.com/a8m/envsubst/releases/download/v1.2.0/envsubst-"$(uname -s)"-"$(uname -m)"
chmod +x "$ENVSUBST"

# Generate cluster resources.
kustomize build "${SOURCE_DIR}/ippool" | "$ENVSUBST" > "${IPPOOL_GENERATED_FILE}"
echo "Generated ${IPPOOL_GENERATED_FILE}"

# Get Cert-manager provider components file
curl --fail -Ss -L -o "${COMPONENTS_CERT_MANAGER_GENERATED_FILE}" https://github.com/cert-manager/cert-manager/releases/download/v1.12.3/cert-manager.yaml
echo "Downloaded ${COMPONENTS_CERT_MANAGER_GENERATED_FILE}"

# Generate Cluster API provider components file.
kustomize build "github.com/kubernetes-sigs/cluster-api/config/default/?ref=main" | "$ENVSUBST" > "${COMPONENTS_CLUSTER_API_GENERATED_FILE}"
echo "Generated ${COMPONENTS_CLUSTER_API_GENERATED_FILE}"

# Generate METAL3 Infrastructure Provider components file.
kustomize build "${SOURCE_DIR}/../config/default" | "$ENVSUBST" > "${COMPONENTS_METAL3_GENERATED_FILE}"
echo "Generated ${COMPONENTS_METAL3_GENERATED_FILE}"

# Generate a single provider components file.
kustomize build "${SOURCE_DIR}/provider-components" | envsubst > "${PROVIDER_COMPONENTS_GENERATED_FILE}"
echo "Generated ${PROVIDER_COMPONENTS_GENERATED_FILE}"