all: test check bench

.PHONY: test
test:
	go test -race -v -coverprofile=coverage.txt -covermode=atomic

.PHONY: bench
bench:
	go test -v -bench . -benchmem -run nothing ./...

.PHONY: check
check:
	golangci-lint run

