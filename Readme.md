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

Sorry, not tests :(

## Contributing

Bug reports and pull requests are welcome on GitHub at https://github.com/anycable/anycable.

## License
The gem is available as open source under the terms of the [MIT License](http://opensource.org/licenses/MIT).
