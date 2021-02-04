OUTPUT ?= dist/anycable-go
GOBENCHDIST ?= dist/gobench

export GO111MODULE=on
export GOFLAGS=-mod=vendor

ifdef VERSION
	LD_FLAGS="-s -w -X github.com/anycable/anycable-go/utils.version=$(VERSION)"
else
	COMMIT := $(shell sh -c 'git log --pretty=format:"%h" -n 1 ')
	VERSION := $(shell sh -c 'git tag -l --sort=-version:refname "v*" | head -n1')
	LD_FLAGS="-s -w -X github.com/anycable/anycable-go/utils.sha=$(COMMIT) -X github.com/anycable/anycable-go/utils.version=$(VERSION)"
endif

GOBUILD=go build -ldflags $(LD_FLAGS) -a

MRUBY_VERSION=1.2.0

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
	go build -tags mrb -ldflags $(LD_FLAGS) -o $(GOBENCHDIST) cmd/gobench-cable/main.go

prepare-mruby:
	cd vendor/github.com/mitchellh/go-mruby && \
	MRUBY_COMMIT=$(MRUBY_VERSION) MRUBY_CONFIG=../../../../../../etc/build_config.rb make libmruby.a

upgrade-mruby: clean-mruby prepare-mruby

clean-mruby:
	cd vendor/github.com/mitchellh/go-mruby && \
	rm -rf vendor/mruby

build-all-mruby:
	env $(GOBUILD) -tags mrb -o "dist/anycable-go-$(VERSION)-mrb-macos-amd64" cmd/anycable-go/main.go
	docker run --rm -v $(PWD):/go/src/github.com/anycable/anycable-go -w /go/src/github.com/anycable/anycable-go -e OUTPUT="dist/anycable-go-$(VERSION)-mrb-linux-amd64" amd64/golang:1.11.4 make build

build-clean:
	rm -rf ./dist

build-linux:
	env GOOS=linux   GOARCH=amd64 $(GOBUILD) -o "dist/anycable-go-$(VERSION)-linux-amd64"   cmd/anycable-go/main.go

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

tmp/anycable-go-test:
	go build -tags mrb -o tmp/anycable-go-test cmd/anycable-go/main.go

test-conformance: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable"

test-conformance-ssl: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token --ssl_key=etc/ssl/server.key --ssl_cert=etc/ssl/server.crt --port=8443" --target-url="wss://localhost:8443/cable"

test-conformance-http: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile ANYCABLE_BROADCAST_ADAPTER=http ANYCABLE_HTTP_BROADCAST_SECRET=any_secret bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable"

test-conformance-all: test-conformance test-conformance-ssl test-conformance-http

test-ci: prepare prepare-mruby test test-conformance

prepare:
	BUNDLE_GEMFILE=.circleci/Gemfile bundle install

gen-ssl:
	mkdir -p tmp/ssl
	openssl genrsa -out tmp/ssl/server.key 2048
	openssl req -new -x509 -sha256 -key tmp/ssl/server.key -out tmp/ssl/server.crt -days 3650

vet:
	go vet ./...

fmt:
	go fmt ./...

lint:
	golangci-lint run

.PHONY: tmp/anycable-go-test
