PROJECT=gool
ORGANISATION=overline-mining
SOURCE=$(shell find . -name '*.go' | grep -v vendor/)
SOURCE_DIRS = src/node

VERSION=$(shell git describe --tags --always --dirty)
DEB_VER=$(shell git describe --tags --abbrev=0 | cut -c 2-)
DEB_HASH=$(shell git rev-parse HEAD)

all: build-node-linux

build-node-linux:
	./build/env.sh go get -v ./...
	./build/env.sh go build -o build/bin/node ./src/node

clean:
	rm -rf build/_workspace/pkg/ build/_workspace/bin
