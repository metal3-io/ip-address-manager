#!/bin/sh
# Validates renovate.json configuration using renovate-config-validator.
# Requires Node.js 22+ for Renovate v40.

set -eux

IS_CONTAINER="${IS_CONTAINER:-false}"
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
    npx --yes -p renovate renovate-config-validator
else
    "${CONTAINER_RUNTIME}" run --rm \
        --env IS_CONTAINER=TRUE \
        --volume "${PWD}:/workdir:ro,z" \
        --entrypoint sh \
        --workdir "/workdir" \
        docker.io/node:24-alpine \
        /workdir/hack/renovate-validator.sh "$@"
fi
