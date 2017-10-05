[![Build Status](https://travis-ci.org/anycable/anycable-go.svg?branch=master)](https://travis-ci.org/anycable/anycable-go) [![Dependency Status](https://dependencyci.com/github/anycable/anycable-go/badge)](https://dependencyci.com/github/anycable/anycable-go) [![Gitter](https://img.shields.io/badge/gitter-join%20chat%20%E2%86%92-brightgreen.svg)](https://gitter.im/anycable/anycable-go)

# AnyCable-Go WebSocket Server

WebSocket server for [Anycable](https://github.com/anycable/anycable).

## Installation

The easiest way to install AnyCable-Go is to [download](https://github.com/anycable/anycable-go/blob/master/DOWNLOADS.md) a pre-compiled binary.

Or with [Homebrew](https://brew.sh/)

```shell
brew install anycable/anycable/anycable-go
```

Of course, you can install it from source too:

```shell
go get -u -f github.com/anycable/anycable-go
```

## Usage

Run server:

```shell
anycable-go -rpc=0.0.0.0:50051 -redis=redis://localhost:6379/5 -redis_channel=anycable -addr=0.0.0.0:8080 -log
```

You can also provide configuration parameters through the corresponding environment variables (i.e. `RPC`, `REDIS`, etc).

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
[Custom stream callbacks](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -
[Subscription Instance Variables](http://edgeapi.rubyonrails.org/classes/ActionCable/Channel/Streams.html) | -

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/anycable/anycable-go.

## License
The library is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
