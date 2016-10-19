# Anycable Go Server

WebSocket server for [Anycable](https://github.com/anycable/anycable).

## Installation


```shell
go get -u -f github.com/anycable/anycable-go
```

## Usage

Run server:

```shell
anycable-go -rpc=0.0.0.0:50051 -redis=redis://localhost:6379/5 -redischannel=anycable -addr=0.0.0.0:8080 -debug
```

## Build

```shell
make
```

## Testing

Sorry, no tests :(


## ActionCable Compatibility

Feature                  | Status 
-------------------------|--------
Connection Identifiers   | +
Connection Request (cookies, params) | +
Disconnect Handling | _coming soon_
Subscribe to channels | +
Parameterized subscriptions | _coming soon_
Unsubscribe from channels | +
Performing Channel Actions | +
Streaming | +
Usage of the same stream name for different channels | -
Broadcasting | +

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/anycable/anycable-go.

## License
The library is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
