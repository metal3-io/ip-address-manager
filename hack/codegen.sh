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
  api/v1alpha1/zz_generated.*.go"

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
    --volume "${PWD}:/data:rw,z" \
    --entrypoint sh \
    --workdir /data \
    registry.hub.docker.com/library/golang:1.16 \
    /data/hack/codegen.sh
fi;