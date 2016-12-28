# AnyCable-Go WebSocket Server

WebSocket server for [Anycable](https://github.com/anycable/anycable).

## Installation

The easiest way to install AnyCable-Go is to [download](https://github.com/anycable/anycable-go/blob/master/DOWNLOADS.md) a pre-compiled binary.

Of course, you can install it from source too:

```shell
go get -u -f github.com/anycable/anycable-go
```

## Usage

Run server:

```shell
anycable-go -rpc=0.0.0.0:50051 -redis=redis://localhost:6379/5 -redischannel=anycable -addr=0.0.0.0:8080 -debug
```

You can also provide configuration parameters through the corresponding environment variables (i.e. `RPC`, `REDIS`, etc).

## Build

```shell
make
```

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
