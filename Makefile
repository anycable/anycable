ifdef VERSION
else
	VERSION := $(shell sh -c 'git describe --always --tags')
endif

OUTPUT ?= dist/anycable-go

ifdef GOBIN
PATH := $(GOBIN):$(PATH)
else
PATH := $(subst :,/bin:,$(GOPATH))/bin:$(PATH)
endif

export GO111MODULE=on
export GOFLAGS=-mod=vendor

LD_FLAGS="-s -w -X main.version=$(VERSION)"
GOBUILD=go build -ldflags $(LD_FLAGS) -a

# Standard build
default: build

# Install current version
install:
	go install ./...

install-with-mruby:
	go install -tags mrb ./...

build:
	go build -tags mrb -ldflags $(LD_FLAGS) -o $(OUTPUT) cmd/anycable-go/main.go

build-gobench:
	go build -tags mrb -ldflags $(LD_FLAGS) -o dist/gobench-cable cmd/gobench-cable/main.go

prepare-mruby:
	cd vendor/github.com/mitchellh/go-mruby && \
	MRUBY_CONFIG=../../../../../../etc/build_config.rb make libmruby.a

build-all-mruby:
	env $(GOBUILD) -tags mrb -o "dist/anycable-go-$(VERSION)-mrb-macos-amd64" cmd/anycable-go/main.go
	docker run --rm -v $(PWD):/go/src/github.com/anycable/anycable-go -w /go/src/github.com/anycable/anycable-go -e OUTPUT="dist/anycable-go-$(VERSION)-mrb-linux-amd64" amd64/golang:1.11.4 make build

build-clean:
	rm -rf ./dist

build-linux:
	env GOOS=linux   GOARCH=386   $(GOBUILD) -o "dist/anycable-go-$(VERSION)-linux-386"     cmd/anycable-go/main.go

build-all: build-clean build-linux
	env GOOS=linux   GOARCH=arm   $(GOBUILD) -o "dist/anycable-go-$(VERSION)-linux-arm"     cmd/anycable-go/main.go
	env GOOS=linux   GOARCH=arm64 $(GOBUILD) -o "dist/anycable-go-$(VERSION)-linux-arm64"   cmd/anycable-go/main.go
	env GOOS=linux   GOARCH=amd64 $(GOBUILD) -o "dist/anycable-go-$(VERSION)-linux-amd64"   cmd/anycable-go/main.go
	env GOOS=windows GOARCH=386   $(GOBUILD) -o "dist/anycable-go-$(VERSION)-win-386"       cmd/anycable-go/main.go
	env GOOS=windows GOARCH=amd64 $(GOBUILD) -o "dist/anycable-go-$(VERSION)-win-amd64"     cmd/anycable-go/main.go
	env GOOS=darwin  GOARCH=386   $(GOBUILD) -o "dist/anycable-go-$(VERSION)-macos-386"     cmd/anycable-go/main.go
	env GOOS=darwin  GOARCH=amd64 $(GOBUILD) -o "dist/anycable-go-$(VERSION)-macos-amd64"   cmd/anycable-go/main.go
	env GOOS=freebsd GOARCH=arm   $(GOBUILD) -o "dist/anycable-go-$(VERSION)-freebsd-arm"   cmd/anycable-go/main.go
	env GOOS=freebsd GOARCH=386   $(GOBUILD) -o "dist/anycable-go-$(VERSION)-freebsd-386"   cmd/anycable-go/main.go
	env GOOS=freebsd GOARCH=amd64 $(GOBUILD) -o "dist/anycable-go-$(VERSION)-freebsd-amd64" cmd/anycable-go/main.go

# Run server
run:
	go run ./cmd/anycable-go/main.go

build-protos:
	protoc --proto_path=./etc --go_out=plugins=grpc:./protos ./etc/rpc.proto

test:
	go test -tags mrb ./...

test-conformance:
	go build -o tmp/anycable-go-test cmd/anycable-go/main.go
	anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable"
	anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token --ssl_key=etc/ssl/server.key --ssl_cert=etc/ssl/server.crt --port=8443" --target-url="wss://localhost:8443/cable"

test-ci: prepare prepare-mruby test test-conformance

prepare:
	gem install anyt

gen-ssl:
	mkdir -p tmp/ssl
	openssl genrsa -out tmp/ssl/server.key 2048
	openssl req -new -x509 -sha256 -key tmp/ssl/server.key -out tmp/ssl/server.crt -days 3650

vet:
	go vet ./...

fmt:
	go fmt ./...
