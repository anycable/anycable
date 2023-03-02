OUTPUT ?= dist/anycable-go
GOBENCHDIST ?= dist/gobench

export GO111MODULE=on

MODIFIER ?= ""

ifndef ANYCABLE_DEBUG
  export ANYCABLE_DEBUG=1
endif

BUILD_ARGS ?=
TEST_FLAGS=
TEST_BUILD_FLAGS=

ifdef COVERAGE
  TEST_FLAGS=-coverprofile=coverage.out
  TEST_BUILD_FLAGS=-cover
	BUILD_ARGS += -cover
endif

# If port 6379 is listening, we assume that this is a Redis instance,
# so we can use a Redis broadcast adapter.
# Otherwise we fallback to HTTP adapter.
ifndef ANYCABLE_BROADCAST_ADAPTER
	ifndef REDIS_URL
		HAS_REDIS := $(shell lsof -Pi :6379 -sTCP:LISTEN -t >/dev/null; echo $$?)
		ifneq ($(HAS_REDIS), 0)
			export ANYCABLE_BROADCAST_ADAPTER=http
		endif
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

current_dir = $(shell pwd)
MRUBY_CONFIG ?= $(current_dir)/etc/build_config.rb

# Standard build
default: build

# Install current version
install:
	go install ./...

install-with-mruby:
	go install -tags mrb ./...

build:
	go build $(BUILD_ARGS) -tags "mrb gops" -ldflags $(LD_FLAGS) -o $(OUTPUT) cmd/anycable-go/main.go

build-gobench:
	go build -tags "mrb gops" -ldflags $(LD_FLAGS) -o $(GOBENCHDIST) cmd/gobench-cable/main.go

download-mruby:
	go mod download github.com/mitchellh/go-mruby

prepare-mruby: download-mruby
	cd $$(go list -m -f '{{.Dir}}' github.com/mitchellh/go-mruby) && \
	MRUBY_COMMIT=$(MRUBY_VERSION) MRUBY_CONFIG=$(MRUBY_CONFIG) make libmruby.a || \
	(sed -i '' 's/{ :verbose => $$verbose }/verbose: $$verbose/g' ./mruby-build/mruby/Rakefile && \
		MRUBY_COMMIT=$(MRUBY_VERSION) MRUBY_CONFIG=$(MRUBY_CONFIG) make libmruby.a)

upgrade-mruby: clean-mruby prepare-mruby

clean-mruby:
	cd $$(go list -m -f '{{.Dir}}' github.com/mitchellh/go-mruby) && \
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
	go test -count=1 -timeout=30s -race -tags mrb ./... $(TEST_FLAGS)

benchmarks: build
	BUNDLE_GEMFILE=.circleci/Gemfile ruby features/runner.rb features/*.benchfile

tmp/anycable-go-test:
	go build $(TEST_BUILD_FLAGS) -tags mrb -race -o tmp/anycable-go-test cmd/anycable-go/main.go

test-conformance: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable"

test-conformance-ssl: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token --ssl_key=etc/ssl/server.key --ssl_cert=etc/ssl/server.crt --port=8443" --target-url="wss://localhost:8443/cable"

test-conformance-http: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile ANYCABLE_BROADCAST_ADAPTER=http ANYCABLE_HTTP_BROADCAST_SECRET=any_secret bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable"

test-conformance-nats: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile ANYCABLE_BROADCAST_ADAPTER=nats bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable"

test-conformance-nats-embedded: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile ANYCABLE_BROADCAST_ADAPTER=nats ANYCABLE_NATS_SERVERS=nats://127.0.0.1:4242 ANYCABLE_EMBED_NATS=true ANYCABLE_ENATS_ADDR=nats://127.0.0.1:4242 bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable"

test-conformance-http-broker: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile ANYCABLE_BROKER=memory ANYCABLE_BROADCAST_ADAPTER=http ANYCABLE_HTTP_BROADCAST_SECRET=any_secret bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable" --require=etc/anyt/broker_tests/*.rb

test-conformance-broker-redis-pubsub: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile ANYCABLE_BROKER=memory ANYCABLE_BROADCAST_ADAPTER=http ANYCABLE_HTTP_BROADCAST_SECRET=any_secret ANYCABLE_PUBSUB=redis bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable" --require=etc/anyt/broker_tests/*.rb

test-conformance-broker-nats-pubsub: tmp/anycable-go-test
	BUNDLE_GEMFILE=.circleci/Gemfile ANYCABLE_BROKER=memory ANYCABLE_EMBED_NATS=true ANYCABLE_ENATS_ADDR=nats://127.0.0.1:4343 ANYCABLE_PUBSUB=nats ANYCABLE_BROADCAST_ADAPTER=http ANYCABLE_HTTP_BROADCAST_SECRET=any_secret bundle exec anyt -c "tmp/anycable-go-test --headers=cookie,x-api-token" --target-url="ws://localhost:8080/cable" --require=etc/anyt/broker_tests/*.rb

test-conformance-all: test-conformance test-conformance-ssl test-conformance-http

TESTFILE ?= features/*.testfile
test-features: build
	BUNDLE_GEMFILE=.circleci/Gemfile ruby features/runner.rb $(TESTFILE)

test-ci: prepare prepare-mruby test test-conformance

prepare:
	BUNDLE_GEMFILE=.circleci/Gemfile bundle install

gen-ssl:
	mkdir -p tmp/ssl
	openssl genrsa -out tmp/ssl/server.key 2048
	openssl req -new -x509 -sha256 -key tmp/ssl/server.key -out tmp/ssl/server.crt -days 3650

bin/golangci-lint:
	@test -x $$(go env GOPATH)/bin/golangci-lint || \
		curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b $$(go env GOPATH)/bin v1.48.0

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
