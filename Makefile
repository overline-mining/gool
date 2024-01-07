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
	./build/env.sh go get -v ./...
	./build/env.sh go build -race -o build/bin/node ./src/node
	./build/env.sh go build -race -o build/bin/btc-rover ./src/rovers/btc/btc-rover.go

build-node-linux:
	./build/env.sh go get -v ./...
	./build/env.sh go build -o build/bin/node ./src/node
	./build/env.sh go build -o build/bin/btc-rover ./src/rovers/btc/btc-rover.go

clean:
	rm -rf build/_workspace/pkg/ build/_workspace/bin
