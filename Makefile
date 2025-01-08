PROJECT=gool
ORGANISATION=overline-mining
SOURCE=$(shell find . -name '*.go' | grep -v vendor/)
SOURCE_DIRS = src/node

VERSION=$(shell git describe --tags --always --dirty)
DEB_VER=$(shell git describe --tags --abbrev=0 | cut -c 2-)
DEB_HASH=$(shell git rev-parse HEAD)

all: build-node-linux

dev: build-node-linux-test

lint:
	./build/env.sh go fmt ./...

build-node-linux-test:
	GOOS="" GOARCH="" GOARM="" ./build/env.sh go install golang.org/x/tools/cmd/goyacc@latest
	GOOS="" GOARCH="" GOARM="" ./build/env.sh go install github.com/blynn/nex@latest
	./build/env.sh go get -v ./...
	./build/env.sh go build -race -o build/bin/node ./src/node

build-node-linux:
	GOOS="" GOARCH="" GOARM="" ./build/env.sh go install golang.org/x/tools/cmd/goyacc@latest
	GOOS="" GOARCH="" GOARM="" ./build/env.sh go install github.com/blynn/nex@latest
	./build/env.sh go get -v ./...
	./build/env.sh go build -o build/bin/node ./src/node

clean:
	rm -rf build/_workspace/pkg/ build/_workspace/bin
