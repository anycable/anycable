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

build-all:
	rm -rf ./dist
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-linux-arm" .
	env CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-linux-arm64" .
	env CGO_ENABLED=0 GOOS=linux GOARCH=386 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-linux-386" .
	env CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-linux-amd64" .
	env CGO_ENABLED=0 GOOS=windows GOARCH=386 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-win-386" .
	env CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-win-amd64" .
	env CGO_ENABLED=0 GOOS=darwin GOARCH=arm go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-macos-arm" .
	env CGO_ENABLED=0 GOOS=darwin GOARCH=386 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-macos-386" .
	env CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-macos-amd64" .
	env CGO_ENABLED=0 GOOS=freebsd GOARCH=arm go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-freebsd-arm" .
	env CGO_ENABLED=0 GOOS=freebsd GOARCH=386 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-freebsd-386" .
	env CGO_ENABLED=0 GOOS=freebsd GOARCH=amd64 go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o "dist/anycable-go-$(VERSION)-freebsd-amd64" .

s3-deploy:
	aws s3 sync --acl=public-read ./dist "s3://anycable/builds/$(VERSION)"

downloads-md:
	ruby etc/generate_downloads.rb

release: build-all s3-deploy downloads-md

dockerize:
	CGO_ENABLED=0 GOOS=linux go build -ldflags "-X main.version=$(VERSION)" -a -installsuffix cgo -o .docker/main .
	docker build -t anycable/anycable-go .

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
