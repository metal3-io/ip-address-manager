#!/bin/sh

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
ARTIFACTS="${ARTIFACTS:-/tmp}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  export XDG_CACHE_HOME=/tmp/.cache
  eval "$(go env)"
  INPUT_FILES="\
  config/certmanager/*.yaml
  config/crd/*.yaml
  config/crd/bases/*.yaml
  config/crd/patches/*.yaml
  config/default/*.yaml
  config/manager/*.yaml
  config/rbac/*.yaml
  config/webhook/*.yaml
  api/v1alpha1/zz_generated.*.go
  ipam/mocks/zz_generated.*.go"

  # shellcheck disable=SC2086
  cksum $INPUT_FILES > "$ARTIFACTS/lint.cksums.before"
  export VERBOSE="--verbose"
  make generate
  # shellcheck disable=SC2086
  cksum $INPUT_FILES > "$ARTIFACTS/lint.cksums.after"
  diff "$ARTIFACTS/lint.cksums.before" "$ARTIFACTS/lint.cksums.after"
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/metal3-ipam:rw,z" \
    --entrypoint sh \
    --workdir /metal3-ipam \
    registry.hub.docker.com/library/golang:1.17 \
    /metal3-ipam/hack/codegen.sh
fi;