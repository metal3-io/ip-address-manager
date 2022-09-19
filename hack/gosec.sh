#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME="${CONTAINER_RUNTIME:-podman}"

if [ "${IS_CONTAINER}" != "false" ]; then
  export XDG_CACHE_HOME="/tmp/.cache"

  gosec -exclude=G107 -severity medium -confidence medium -quiet -concurrency 16 ./...
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/metal3-ipam:ro,z" \
    --entrypoint sh \
    --workdir /metal3-ipam \
    registry.hub.docker.com/securego/gosec:latest \
    /metal3-ipam/hack/gosec.sh
fi;