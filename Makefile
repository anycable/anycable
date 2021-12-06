OUTPUT ?= dist/anycable-go
GOBENCHDIST ?= dist/gobench

export GO111MODULE=on
export GOFLAGS=-mod=vendor

MODIFIER ?= ""

ifndef ANYCABLE_DEBUG
  export ANYCABLE_DEBUG=1
endif

# If port 6379 is listening, we assume that this is a Redis instance,
# so we can use a Redis broadcast adapter.
# Otherwise we fallback to HTTP adapter.
ifndef REDIS_URL
	HAS_REDIS := $(shell lsof -Pi :6379 -sTCP:LISTEN -t >/dev/null; echo $$?)
	ifneq ($(HAS_REDIS), 0)
		export ANYCABLE_BROADCAST_ADAPTER=http
	endif
endif

ifdef VERSION
	LD_FLAGS="-s -w -X github.com/anycable/anycable-go/version.version=$(VERSION) -X github.com/anycable/anycable-go/version.modifier=$(MODIFIER)"
else
	COMMIT := $(shell sh -c 'git log --pretty=format:"%h" -n 1 ')
	VERSION := $(shell sh -c 'git tag -l --sort=-version:refname "v*" | head -n1')
	LD_FLAGS="-s -w -X github.com/anycable/anycable-go/version.sha=$(COMMIT) -X github.com/anycable/anycable-go/utils.version=$(VERSION) -X github.com/anycable/anycable-go/version.modifier=$(MODIFIER)"
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
	go build -tags "mrb gops" -ldflags $(LD_FLAGS) -o $(OUTPUT) cmd/anycable-go/main.go

build-gobench:
	go build -tags "mrb gops" -ldflags $(LD_FLAGS) -o $(GOBENCHDIST) cmd/gobench-cable/main.go

vendor:
	mv vendor/github.com/mitchellh/go-mruby/mruby-build tmp/
	mv vendor/github.com/mitchellh/go-mruby/libmruby.a tmp/
	go mod vendor
	mv tmp/mruby-build vendor/github.com/mitchellh/go-mruby/
	mv tmp/libmruby.a vendor/github.com/mitchellh/go-mruby/libmruby.a

prepare-mruby:
	cd vendor/github.com/mitchellh/go-mruby && \
	MRUBY_COMMIT=$(MRUBY_VERSION) MRUBY_CONFIG=../../../../../../etc/build_config.rb make libmruby.a || \
	(sed -i '' 's/{ :verbose => $$verbose }/verbose: $$verbose/g' ./mruby-build/mruby/Rakefile && \
		MRUBY_COMMIT=$(MRUBY_VERSION) MRUBY_CONFIG=../../../../../../etc/build_config.rb make libmruby.a)

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
	go run -ldflags $(LD_FLAGS) -tags "mrb gops" ./cmd/anycable-go/main.go

build-protos:
	protoc --proto_path=./etc --go_out=plugins=grpc:./protos ./etc/rpc.proto

bench:
	go test -tags mrb -bench=. ./...

test:
	go test -count=1 -timeout=30s -race -tags mrb ./...

benchmarks: build
	BUNDLE_GEMFILE=.circleci/Gemfile ruby benchmarks/runner.rb benchmarks/*.benchfile

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


bin/shadow:
	@which shadow &> /dev/null || \
		env GO111MODULE=off go get golang.org/x/tools/go/analysis/passes/shadow/cmd/shadow

bin/golangci-lint:
	@test -x $$(go env GOPATH)/bin/golangci-lint || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.39.0

bin/gosec:
	@test -x $$(go env GOPATH)/bin/gosec || \
		curl -sfL https://raw.githubusercontent.com/securego/gosec/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v2.7.0

vet: bin/shadow
	go vet ./...
	go vet -vettool=$$(which shadow) ./...

sec: bin/gosec
	$$(go env GOPATH)/bin/gosec -quiet -confidence=medium -severity=medium  ./...

lint: bin/golangci-lint
	$$(go env GOPATH)/bin/golangci-lint run

fmt:
	go fmt ./...

bin/go-licenses:
	@which go-licenses &> /dev/null || \
		env GO111MODULE=off go get -v github.com/google/go-licenses

licenses: bin/go-licenses
	@env GOFLAGS="-tags=mrb" $$(go env GOPATH)/bin/go-licenses csv github.com/anycable/anycable-go/cli 2>/dev/null | awk -F',' '{ print $$3 }' | sort | uniq | grep -v "Unknown"
	@env GOFLAGS="-tags=mrb" $$(go env GOPATH)/bin/go-licenses csv github.com/anycable/anycable-go/cli 2>/dev/null | grep "Unknown" | grep -v "anycable-go" || echo "No unknown licenses 👌"

.PHONY: tmp/anycable-go-test vendor
