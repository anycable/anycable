# Getting Started with AnyCable-Go

AnyCable-Go is a WebSocket server for AnyCable written in Golang.

## Installation

The easiest way to install AnyCable-Go is to [download](https://github.com/anycable/anycable-go/releases) a pre-compiled binary.

MacOS users could install it with [Homebrew](https://brew.sh/)

```sh
brew install anycable-go

# or use --HEAD option for edge versions
brew install anycable-go --HEAD
```

Arch Linux users can install [anycable-go package from AUR](https://aur.archlinux.org/packages/anycable-go/).

Of course, you can install it from source too:

```sh
go get -u -f github.com/anycable/anycable-go/cmd/anycable-go
```

### Via NPM

For Node.js projects, there is also an option to install AnyCable-Go via NPM:

```sh
npm install --save-dev @anycable/anycable-go
pnpm install --save-dev @anycable/anycable-go
yarn add --dev @anycable/anycable-go

# and run as follows
npx anycable-go
```

**NOTE:** The version of the NPM package is the same as the version of the AnyCable-Go binary (which is downloaded automatically on the first run).

## Usage

Run server:

```sh
$ anycable-go

=> INFO time context=main Starting AnyCable v1.2.1 (pid: 12902, open files limit: 524288, gomaxprocs: 4)
```

By default, `anycable-go` tries to connect to an RPC server listening at `localhost:50051` (the default host for the Ruby gem). You can change this setting by providing `--rpc_host` option or `ANYCABLE_RPC_HOST` env variable (read more about [configuration](./configuration.md)).

All other configuration parameters have the same default values as the corresponding parameters for the AnyCable RPC server, so you don't need to change them usually.
