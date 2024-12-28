# Contributing to AnyCable

Run the following commands to prepare the project:

```shell
# First, prepare mruby (we embed it by default)
# NOTE: Might require running with sudo, since we build artifacts within a Go module
make prepare-mruby

# then build the Go binary (will be available in dist/anycable-go)
make
```

You can run tests with the following commands:

```sh
# Run Golang unit tests
make test

# Run integrations tests (there are various conformance commands, some require Redis)
make test-conformance

# Run integration benchmarks
go install github.com/anycable/websocket-bench@latest
make benchmarks
```

We use [golangci-lint](https://golangci-lint.run) to lint Go source code:

```sh
make lint
```
