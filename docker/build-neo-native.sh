#!/bin/bash

NEO_VERSION="v2.13.0"
REPO_NAME=$1
GOOL_VERSION=$2

mkdir -p $(pwd)/neo-build
cp $(pwd)/docker/Dockerfile.neo-native $(pwd)/neo-build/
pushd $(pwd)/neo-build

git clone https://github.com/neo-project/neo-node.git -b ${NEO_VERSION}
cp Dockerfile.neo-native neo-node/Dockerfile
pushd neo-node

sed -i 's/netcoreapp2.1/netcoreapp3.1/' neo-cli/neo-cli.csproj

docker build -t ${REPO_NAME}/neo:${GOOL_VERSION} -t ${REPO_NAME}/neo:latest .

popd

popd
rm -rf $(pwd)/neo-build