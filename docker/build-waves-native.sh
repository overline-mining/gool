#!/bin/bash

WAVES_VERSION="v1.3.14"
REPO_NAME=$1
GOOL_VERSION=$2

mkdir -p $(pwd)/waves-build
pushd $(pwd)/waves-build

git clone https://github.com/wavesplatform/Waves.git -b ${WAVES_VERSION}
pushd Waves

sed -i 's/buster/unstable/' docker/Dockerfile
./build-with-docker.sh && docker build -t ${REPO_NAME}/waves:${GOOL_VERSION} -t ${REPO_NAME}/waves:latest docker || { echo "WAVES BUILD FAILED"; popd; popd; rm -rf $(pwd)/waves-build; exit 1; } 

popd

popd
rm -rf $(pwd)/waves-build
