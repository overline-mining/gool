#!/bin/bash

GOOL_ROOT=$(pwd)
pushd ../
mkdir -p go
export GOPATH=$(pwd)/go
PATH="$PATH:$(go env GOPATH)/bin"

go get google.golang.org/protobuf/cmd/protoc-gen-go
go get google.golang.org/grpc/cmd/protoc-gen-go-grpc

git clone https://github.com/blockcollider/bcjs.git
cd bcjs/protos
for pb in $(ls *.proto)
do
    sed -i "s/package bcsdk\;/option go_package = \"github.com\/overline\-mining\/gool\/src\/protos\"\;/g" ${pb}
    protoc \
      --go_out=${GOOL_ROOT}/src/protos --go_opt=paths=source_relative \
      --go-grpc_out=${GOOL_ROOT}/src/protos --go-grpc_opt=paths=source_relative \
      ${pb}
done
popd
