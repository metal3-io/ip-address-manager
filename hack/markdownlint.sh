#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  TOP_DIR="${1:-.}"
  find "${TOP_DIR}" -type d \( -path ./vendor -o -path ./.github \) -prune -o -name '*.md' -exec mdl --style all --warnings {} \+
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/metal3-ipam:ro,z" \
    --entrypoint sh \
    --workdir /metal3-ipam \
    registry.hub.docker.com/pipelinecomponents/markdownlint:latest \
    /metal3-ipam/hack/markdownlint.sh "${@}"
fi;
