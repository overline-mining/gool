#!/bin/bash

LISK_VERSION="v3.0.3"
ARCH=$1
REPO_NAME=$2
GOOL_VERSION=$3
DO_PUSH=$4

mkdir -p $(pwd)/lisk-build
cp $(pwd)/docker/Dockerfile.lisk $(pwd)/lisk-build
pushd $(pwd)/lisk-build

git clone https://github.com/LiskHQ/lisk-core.git -b ${LISK_VERSION}
mv Dockerfile.lisk lisk-core/docker/Dockerfile
pushd lisk-core/docker

docker buildx build --platform ${ARCH} --tag ${REPO_NAME}/lisk:${GOOL_VERSION} . ${DO_PUSH}

popd

popd
rm -rf $(pwd)/lisk-build
