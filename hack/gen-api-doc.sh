#!/usr/bin/env bash

set -o errexit -o nounset -o pipefail

CRDOC="${CRDOC:-crdoc}"

"${CRDOC}" --resources config/crd/bases/ --output docs/api.md