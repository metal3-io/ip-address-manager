#!/bin/bash -x

[[ -f bin/kustomize ]] && exit 0

version=4.4.1
arch=$(go env GOARCH)
os=$(go env GOOS)

mkdir -p ./bin
curl -L -O "https://github.com/kubernetes-sigs/kustomize/releases/download/kustomize%2Fv${version}/kustomize_v${version}_${os}_${arch}.tar.gz"

tar -xzvf "kustomize_v${version}_${os}_${arch}.tar.gz"
mv kustomize ./bin

rm "kustomize_v${version}_${os}_${arch}.tar.gz"
