#!/bin/bash

GOOL_ROOT=$(pwd)
pushd ../
mkdir -p go
export GOPATH=$(pwd)/go
PATH="$PATH:$(go env GOPATH)/bin"

go get google.golang.org/protobuf/cmd/protoc-gen-go
go get google.golang.org/grpc/cmd/protoc-gen-go-grpc

git clone https://github.com/blockcollider/overline-proto.git
pushd overline-proto/protos
for pb in $(ls *.proto)
do
    sed -i.old '2s;^;\noption\ go_package\ \=\ \"github.com\/gool\/src\/protos\"\;\n;' ${pb}
done
protoc \
  --go_out=${GOOL_ROOT}/src/protos --go_opt=paths=source_relative \
  --go-grpc_out=${GOOL_ROOT}/src/protos --go-grpc_opt=paths=source_relative \
  *.proto

popd
popd
