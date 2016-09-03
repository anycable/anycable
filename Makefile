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

# Get dependencies and use gdm to checkout changesets
prepare:
  go get github.com/sparrc/gdm
  gdm restore

vet:
  go vet ./...
