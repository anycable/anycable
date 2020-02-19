[![TravisCI](https://img.shields.io/travis/anycable/anycable-go.svg?label=TravisCI&style=flat)](https://travis-ci.org/anycable/anycable-go)
[![CircleCI](https://img.shields.io/circleci/project/github/anycable/anycable-go.svg?label=CircleCI)](https://circleci.com/gh/anycable/anycable-go)
[![Docker](https://img.shields.io/docker/pulls/anycable/anycable-go.svg)](https://hub.docker.com/r/anycable/anycable-go/)
[![Gitter](https://img.shields.io/badge/gitter-join%20chat%20%E2%86%92-brightgreen.svg)](https://gitter.im/anycable/anycable-go)
[![Documentation](https://img.shields.io/badge/docs-link-brightgreen.svg)](https://docs.anycable.io/#go_getting_started)

# AnyCable-Go WebSocket Server

WebSocket server for [AnyCable](https://github.com/anycable/anycable).

**Important** This is a readme for the upcoming v1.0 release.

## Versioning

**Important** Use the same major version of AnyCable-Go as your AnyCable gem.
AnyCable-Go uses the same major version number (and minor version as well for 0.x series) as other libraries to indicate the compatibility.

**NOTE**: AnyCable-Go v1.0 is backward-compatible with v0.6.x series (to allow incremental upgrades). The next minor relase (v1.1) will drop 0.6.x support.

## Installation

The easiest way to install AnyCable-Go is to [download](https://github.com/anycable/anycable-go/releases) a pre-compiled binary.

MacOS users could install it with [Homebrew](https://brew.sh/)

```shell
# NOTE: while v1.0 hasn't been released, you should use --HEAD option
brew install --HEAD anycable-go
```

Arch Linux users can install [anycable-go package from AUR](https://aur.archlinux.org/packages/anycable-go/).

Of course, you can install it from source too:

```shell
go get -u -f github.com/anycable/anycable-go/cmd/anycable-go
```

**NOTE:** right now it's not possible to build `anycable-go` with mruby support using the command above. To install `anycable-go` with mruby from source try:

```
go get -d -u -f github.com/anycable/anycable-go/cmd/anycable-go && (cd $GOPATH/src/github.com/anycable/anycable-go && make prepare-mruby install-with-mruby)
```

## Upgrade

For instructions on how to upgrade to a newer version see [upgrade notes](https://docs.anycable.io/#upgrade_to_1_0_0?id=anycable-go).

### Heroku

See [heroku-anycable-go](https://github.com/anycable/heroku-anycable-go) buildpack.

## Usage

Run server:

```shell
$ anycable-go

=> INFO 2020-02-05T08:44:57.684Z context=main Starting AnyCable 1.0.0
```

You can also provide configuration parameters through the corresponding environment variables (i.e. `ANYCABLE_RPC_HOST`, `ANYCABLE_REDIS_URL`, etc).

For more information about available options run `anycable-go -h`.

📑 [Documentation](https://docs.anycable.io/#/anycable-go/getting_started)

## Build

```shell
# first, prepare mruby (we embed it by default)
make prepare-mruby

# then build the Go binary (will be available in dist/anycable-go)
make
```

## Docker

See available images [here](https://hub.docker.com/r/anycable/anycable-go/).

## Contributing

Bug reports and pull requests are welcome on GitHub at [https://github.com/anycable/anycable-go](https://github.com/anycable/anycable-go).

Please, provide reproduction script (using [this template](https://github.com/anycable/anycable/blob/master/etc/bug_report_template.rb)) when submitting bugs if possible.

## License

The library is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
