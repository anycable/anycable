[![TravisCI](https://travis-ci.org/anycable/anycable-go.svg?branch=master)](https://travis-ci.org/anycable/anycable-go)
[![CircleCI](https://circleci.com/gh/anycable/anycable-go.svg?style=svg)](https://circleci.com/gh/anycable/anycable-go)
[![Dependency Status](https://dependencyci.com/github/anycable/anycable-go/badge)](https://dependencyci.com/github/anycable/anycable-go)
[![Gitter](https://img.shields.io/badge/gitter-join%20chat%20%E2%86%92-brightgreen.svg)](https://gitter.im/anycable/anycable-go)
[![Documentation](https://img.shields.io/badge/docs-link-brightgreen.svg)](https://docs.anycable.io/#go_getting_started)

# AnyCable-Go WebSocket Server

WebSocket server for [AnyCable](https://github.com/anycable/anycable).

**NOTE:** this is a readme for the 0.6.x version. [Go to 0.5.x version](https://github.com/anycable/anycable-go/tree/0-5-stable).

## Installation

The easiest way to install AnyCable-Go is to [download](https://github.com/anycable/anycable-go/releases) a pre-compiled binary.

MacOS users could install it with [Homebrew](https://brew.sh/)

```shell
brew install anycable-go
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

For instructions on how to upgrade to a newer version see [upgrade notes](https://docs.anycable.io/#upgrade_to_0_6_0?id=anycable-go).

### Heroku

See [heroku-anycable-go](https://github.com/anycable/heroku-anycable-go) buildpack.

## Usage

Run server:

```shell
$ anycable-go

=> INFO 2018-03-05T08:44:57.684Z context=main Starting AnyCable 0.6.0
```

You can also provide configuration parameters through the corresponding environment variables (i.e. `ANYCABLE_RPC_HOST`, `ANYCABLE_REDIS_URL`, etc).

For more information about available options run `anycable-go -h`.

📑 [Documentation](https://docs.anycable.io/#go_getting_started)

## Build

```shell
make
```

## Docker

See available images [here](https://hub.docker.com/r/anycable/anycable-go/).

## ActionCable Compatibility

Feature                  | Status
-------------------------|--------
Connection Identifiers   | +
Connection Request (cookies, params) | +
Disconnect Handling | +
Subscribe to channels | +
Parameterized subscriptions | +
Unsubscribe from channels | +
Performing Channel Actions | +
Streaming | +
Usage of the same stream name for different channels | +
Broadcasting | +
Remote disconnect | - (WIP)
[Custom stream callbacks](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -
[Subscription Instance Variables](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/anycable/anycable-go.

Please, provide reproduction script (using [this template](https://github.com/anycable/anycable/blob/master/etc/bug_report_template.rb)) when submitting bugs if possible.

## License
The library is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
