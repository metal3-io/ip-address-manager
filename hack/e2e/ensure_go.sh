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

set -o errexit
set -o nounset
set -o pipefail

MINIMUM_GO_VERSION=go1.25.10

# Ensure the go tool exists and is a viable version, or installs it
verify_go_version()
{
    # If go is not available on the path, get it
    if ! [ -x "$(command -v go)" ]; then
        if [[ "${OSTYPE}" == "linux-gnu" ]]; then
            echo 'go not found, installing'
            curl -sLo "/tmp/${MINIMUM_GO_VERSION}.linux-amd64.tar.gz" "https://go.dev/dl/${MINIMUM_GO_VERSION}.linux-amd64.tar.gz"
            sudo tar -C /usr/local -xzf "/tmp/${MINIMUM_GO_VERSION}.linux-amd64.tar.gz"
            export PATH="/usr/local/go/bin:${PATH}"
        else
            echo "Missing required binary in path: go"
            return 2
        fi
    fi

    local go_version
    IFS=" " read -ra go_version <<< "$(go version)"
    if [[ "${MINIMUM_GO_VERSION}" != $(echo -e "${MINIMUM_GO_VERSION}\n${go_version[2]}" | sort -s -t. -k 1,1 -k 2,2n -k 3,3n | head -n1) ]] && [[ "${go_version[2]}" != "devel" ]]; then
        cat << EOF
Detected go version: ${go_version[2]}.
Requires ${MINIMUM_GO_VERSION} or greater.
Please install ${MINIMUM_GO_VERSION} or later.
EOF
        return 2
    fi
}

verify_go_version
