VERSION := $(shell sh -c 'git describe --always --tags')
ifdef GOBIN
PATH := $(GOBIN):$(PATH)
else
PATH := $(subst :,/bin:,$(GOPATH))/bin:$(PATH)
endif

# Standard build
default: prepare build

# Only run the build (no dependency grabbing)
build:
	go install -ldflags "-X main.version=$(VERSION)" ./...

# Run server
run:
	go run ./*.go

build-protos:
	protoc --proto_path=./etc --go_out=plugins=grpc:./protos ./etc/rpc.proto

# Get dependencies and use gdm to checkout changesets
prepare:
	go get github.com/tools/godep
	godep restore

vet:
	go vet ./...

fmt:
	go fmt ./...
