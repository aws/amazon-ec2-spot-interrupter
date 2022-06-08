BUILD_DIR ?= $(dir $(realpath -s $(firstword $(MAKEFILE_LIST))))/build
VERSION ?= $(shell git describe --tags --always --dirty)
GOOS ?= $(shell uname | tr '[:upper:]' '[:lower:]')
GOARCH ?= $(shell [[ `uname -m` = "x86_64" ]] && echo "amd64" || echo "arm64" )
GOPROXY ?= "https://proxy.golang.org,direct"

$(shell mkdir -p ${BUILD_DIR})

all: verify test build

build:
	go build -a -ldflags="-s -w -X main.version=${VERSION}" -o ${BUILD_DIR}/itn-${GOOS}-${GOARCH} ${BUILD_DIR}/../cmd/main.go

test:
	go test -bench=. ${BUILD_DIR}/../... -v -coverprofile=coverage.out -covermode=atomic -outputdir=${BUILD_DIR}

e2e-test:
	go build -a -ldflags="-s -w -X main.version=${VERSION}" -o ${BUILD_DIR}/spot-itn ${BUILD_DIR}/../cmd/main.go
	go test ./test/e2e -v

verify:
	go mod tidy
	go mod download
	go vet ./...
	go fmt ./...

version:
	@echo ${VERSION}

help:
	@grep -E '^[a-zA-Z_-]+:.*$$' $(MAKEFILE_LIST) | sort

.PHONY: all build test verify help