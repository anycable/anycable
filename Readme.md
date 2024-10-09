[![Latest Release](https://img.shields.io/github/release/anycable/anycable-go.svg?include_prereleases)](https://github.com/anycable/anycable-go/releases/latest?include_prereleases)
[![Build](https://github.com/anycable/anycable-go/workflows/Test/badge.svg)](https://github.com/anycable/anycable-go/actions)
[![Docker](https://img.shields.io/docker/pulls/anycable/anycable-go.svg)](https://hub.docker.com/r/anycable/anycable-go/)
[![Documentation](https://img.shields.io/badge/docs-link-brightgreen.svg)](https://docs.anycable.io/anycable-go/getting_started)
# AnyCable WebSocket Server

A real-time server component of [AnyCable](https://anycable.io) (open-source edition). Check out also our
[Pro](https://docs.anycable.io/pro) and [managed](https://plus.anycable.io) offerings.

> [!NOTE]
> You can find all the necessary information about AnyCable in our documentation: [docs.anycable.io](https://docs.anycable.io).

## Installation

There are several ways to install AnyCable server:

- On MacOS, you can install AnyCable via [Homebrew](https://brew.sh/):

  ```shell
  brew install anycable-go
  ```

- Docker images are available on [Docker Hub](https://hub.docker.com/r/anycable/anycable-go/).

- For Rails projects, we recommend using our `bin/rails g anycable:bin` installer for local development.

- For JavaScript projects, we recommend installing AnyCable via NPM:

  ```sh
  npm install --save-dev @anycable/anycable-go
  pnpm install --save-dev @anycable/anycable-go
  yarn add --dev @anycable/anycable-go

  # and run as follows
  npx anycable-go
  ```

- You can use [heroku-anycable-go](https://github.com/anycable/heroku-anycable-go) buildpack for Heroku deployments.

- Arch Linux users can install [anycable-go package from AUR](https://aur.archlinux.org/packages/anycable-go/).

- Or you can download a binary from the [releases page](https://github.com/anycable/anycable-go/releases):

  ```sh
  # Example for `anycable-go-linux-amd64`
  curl -fsSL https://github.com/anycable/anycable-go/releases/latest/download/anycable-go-linux-amd64 -o anycable-go
  chmod +x anycable-go
  ./anycable-go -v
  ```

- Of course, you can install it from source too:

  ```shell
  go install github.com/anycable/anycable-go/cmd/anycable-go@latest
  ```

## Usage

Run server:

```shell
$ anycable-go

2024-10-09 11:00:01.402 INF Starting AnyCable 1.5.3-f39ff3f (pid: 85844, open file limit: 122880, gomaxprocs: 8) nodeid=E4eFyM
```

For more information about available options run `anycable-go -h` or check out [the documentation](https://docs.anycable.io/anycable-go/configuration).

## Build

```shell
# first, prepare mruby (we embed it by default)
# NOTE: Might require running with sudo, since we build artifacts within a Go module
make prepare-mruby

# then build the Go binary (will be available in dist/anycable-go)
make
```

You can run tests with the following commands:

```sh
# Run Golang unit tests
make test

# Run once
make prepare

# Run integrations tests
make test-conformance

# Run integration benchmarks
go install github.com/anycable/websocket-bench@latest
make benchmarks
```

We use [golangci-lint](https://golangci-lint.run) to lint Go source code:

```sh
make lint
```

## Contributing

Bug reports and pull requests are welcome on GitHub at [https://github.com/anycable/anycable-go](https://github.com/anycable/anycable-go).

Please, provide reproduction script (using [this template](https://github.com/anycable/anycable/blob/master/etc/bug_report_template.rb)) when submitting bugs if possible.

## License

The library is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).

## Security Contact

To report a security vulnerability, please contact us at `anycable@evilmartians.com`. We will coordinate the fix and disclosure.
