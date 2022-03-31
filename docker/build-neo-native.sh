#!/bin/bash

NEO_VERSION="v2.13.0"
REPO_NAME=$1
GOOL_VERSION=$2

mkdir -p $(pwd)/neo-build
cp $(pwd)/docker/Dockerfile.neo-native $(pwd)/neo-build/
cp $(pwd)/docker/install-neo-plugins.sh $(pwd)/neo-build/
cp $(pwd)/docker/neo-config.json $(pwd)/neo-build/
pushd $(pwd)/neo-build

git clone https://github.com/neo-project/neo-node.git -b ${NEO_VERSION}
cp Dockerfile.neo-native neo-node/Dockerfile
cp install-neo-plugins.sh neo-node/
cp neo-config.json neo-node/
pushd neo-node

sed -i 's/netcoreapp2.1/netcoreapp3.1/' neo-cli/neo-cli.csproj

docker build --build-arg NEO_VERSION=${NEO_VERSION} -t ${REPO_NAME}/neo:${GOOL_VERSION} -t ${REPO_NAME}/neo:latest . || { echo "NEO BUILD FAILED"; popd; popd; rm -rf $(pwd)/neo-build; exit 1; }

popd

popd
rm -rf $(pwd)/neo-build
