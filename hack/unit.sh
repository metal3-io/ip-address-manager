#!/bin/sh
# shellcheck disable=SC2292

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"
WORKDIR="${WORKDIR:-/workdir}"

if [ "${IS_CONTAINER}" != "false" ]; then
    export XDG_CACHE_HOME=/tmp/.cache
    UNIT_DIR="$(mktemp -d -t unit.XXXXXX)"
    # This part is run outside of container and generated files should be
    # cleaned up just in case this is run locally and not in test environment.
    trap 'rm -rf "${UNIT_DIR}"' EXIT
    cp -r ./* "${UNIT_DIR}"
    cd "${UNIT_DIR}"
    make unit
else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:${WORKDIR}:ro,z" \
        --entrypoint sh \
        --workdir "${WORKDIR}" \
        docker.io/golang:1.26 \
        "${WORKDIR}"/hack/unit.sh "$@"
fi
