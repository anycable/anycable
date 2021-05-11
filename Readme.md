[![Latest Release](https://img.shields.io/github/release/anycable/anycable-go.svg?include_prereleases)](https://github.com/anycable/anycable-go/releases/latest?include_prereleases)
[![Build](https://github.com/anycable/anycable-go/workflows/Test/badge.svg)](https://github.com/anycable/anycable-go/actions)
[![Docker](https://img.shields.io/docker/pulls/anycable/anycable-go.svg)](https://hub.docker.com/r/anycable/anycable-go/)
[![Documentation](https://img.shields.io/badge/docs-link-brightgreen.svg)](https://docs.anycable.io/anycable-go/pro)
# AnyCable-Go WebSocket Server <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' />

WebSocket server for [AnyCable](https://github.com/anycable/anycable).

> [AnyCable Pro](https://docs.anycable.io/pro) has been launched 🚀

## Versioning

**Important** Use the same major version of AnyCable-Go as your AnyCable gem.
AnyCable-Go uses the same major version number (and minor version as well for 0.x series) as other libraries to indicate the compatibility.

## Installation

The easiest way to install AnyCable-Go is to [download](https://github.com/anycable/anycable-go/releases) a pre-compiled binary:

```shell
# Example for `anycable-go-linux-amd64`
curl -fsSL https://github.com/anycable/anycable-go/releases/latest/download/anycable-go-linux-amd64 -o anycable-go
chmod +x anycable-go
./anycable-go -v
```

MacOS users could install it with [Homebrew](https://brew.sh/)

```shell
brew install anycable-go
```

Arch Linux users can install [anycable-go package from AUR](https://aur.archlinux.org/packages/anycable-go/).

Of course, you can install it from source too:

```shell
go install github.com/anycable/anycable-go/cmd/anycable-go@latest
```

For JavaScript projects, there is also an option to install AnyCable-Go via NPM:

```sh
npm install --save-dev @anycable/anycable-go
pnpm install --save-dev @anycable/anycable-go
yarn add --dev @anycable/anycable-go

# and run as follows
npx anycable-go
```

## Upgrade

For instructions on how to upgrade to a newer version see [upgrade notes](https://docs.anycable.io/upgrade-notes/Readme.md).

### Heroku

See [heroku-anycable-go](https://github.com/anycable/heroku-anycable-go) buildpack.

## Usage

Run server:

```shell
$ anycable-go

=> INFO 2020-02-05T08:44:57.684Z context=main Starting AnyCable 1.1.0
```

You can also provide configuration parameters through the corresponding environment variables (i.e. `ANYCABLE_RPC_HOST`, `ANYCABLE_REDIS_URL`, etc).

For more information about available options run `anycable-go -h`.

📑 [Documentation](https://docs.anycable.io/anycable-go/getting_started)

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

## Docker

See available images [here](https://hub.docker.com/r/anycable/anycable-go/).

## Contributing

Bug reports and pull requests are welcome on GitHub at [https://github.com/anycable/anycable-go](https://github.com/anycable/anycable-go).

Please, provide reproduction script (using [this template](https://github.com/anycable/anycable/blob/master/etc/bug_report_template.rb)) when submitting bugs if possible.

## License

The library is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).

## Security Contact

To report a security vulnerability, please contact us at `anycable@evilmartians.com`. We will coordinate the fix and disclosure.
