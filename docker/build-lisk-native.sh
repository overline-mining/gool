#!/bin/bash

LISK_VERSION="v3.0.3"
REPO_NAME=$1
GOOL_VERSION=$2

mkdir -p $(pwd)/lisk-build
cp $(pwd)/docker/Dockerfile.lisk $(pwd)/lisk-build
pushd $(pwd)/lisk-build

git clone https://github.com/LiskHQ/lisk-core.git -b ${LISK_VERSION}
mv Dockerfile.lisk lisk-core/docker/Dockerfile
pushd lisk-core/docker

docker build -t ${REPO_NAME}/lisk:${GOOL_VERSION} -t ${REPO_NAME}/lisk:latest .

popd

popd
rm -rf $(pwd)/lisk-build
