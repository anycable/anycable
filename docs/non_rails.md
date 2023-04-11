# Using AnyCable without Rails

AnyCable can be used without Rails, thus allowing you to use ActionCable-like functionality in your app.

> Learn how to use AnyCable with Hanami in the ["AnyCable off Rails: connecting Twilio streams with Hanami"](https://evilmartians.com/chronicles/anycable-goes-off-rails-connecting-twilio-streams-with-hanami) blog post.

## Requirements

- Ruby >= 2.7
- Redis or NATS (see [broadcast adapters](broadcast_adapters.md))

## Installation

Add `anycable` gem to your `Gemfile`:

```ruby
gem "anycable", "~> 1.1"

# when using Redis broadcast adapter
gem "redis", ">= 4.0"

# when using NATS broadcast adapter
gem "nats-pure", "~> 2"
```

(and don't forget to run `bundle install`).

Now you need to add _channels_ layer to your application (`anycable` gem only provides a [CLI](./cli.md) and a gRPC server).

You can use an existing solution (e.g., `litecable`) or build your own.

## Lite Cable

There is a ready-to-go framework – [Lite Cable](https://github.com/palkan/litecable) – which can be used for application logic. It also supports AnyCable out-of-the-box.

Resources:

- [Lite Cable docs](https://github.com/palkan/litecable)
- [Lite Cable Sinatra example](https://github.com/palkan/litecable/tree/master/examples/sinatra)
- [Connecting LiteCable to Hanami](http://gabrielmalakias.com.br/ruby/hanami/iot/2017/05/26/websockets-connecting-litecable-to-hanami.html) by [@GabrielMalakias](https://github.com/GabrielMalakias).

## Custom Ruby framework

You can build your own framework to use as _logic-handler_ for AnyCable.

AnyCable initiates a _connection_ object for every request using user-provided factory:

```ruby
# Specify factory
AnyCable.connection_factory = MyConnectionFactory

# And then AnyCable calls .call method on your factory
connection = factory.call(socket, **options)
```

Where:

- `socket` – is an object, representing client's socket (say, _socket stub_) (see [socket.rb](https://github.com/anycable/anycable/blob/master/lib/anycable/socket.rb))
- `options` may contain:
   - `identifiers`: a JSON string returned by `connection.identifiers_json` on connection (see below)
   - `subscriptions`: a list of channels identifiers for the connection.

Connection interface:

```ruby
class Connection
  # Called on connection
  def handle_open
  end

  # Called on disconnection
  def handle_close
  end

  # Called on incoming message.
  # Client send a JSON-encoded message of the form { "identifier": ..., "command": ..., "data" ... }.
  # - identifier – channel identifier (e.g. `{"channel":"chat","id":1}`)
  # - command – e.g. "subscribe", "unsubscribe", "message"
  # - any additional data
  def handle_channel_command(identifier, command, data)
    # ...
  end

  # Returns any string which can be used later in .create function to initiate connection.
  def identifiers_json
  end
end
```

`Connection#handle_channel_command` should return truthy value on success (i.e., when a subscription is confirmed, or action is called).

*NOTE*: connection instance is initiated on every request, so it should be stateless (except `identifiers_json`).

To send a message to a client, you should call `socket#transmit`.
For manipulating with streams use `socket#subscribe`, `socket#unsubscribe` and `socket#unsubscribe_from_all`.

To persist client states between RPC calls you can use `socket#cstate` (connection state) and `socket#istate` (per-channel state), which are key-value stores (keys and values must both be strings).

See [test factory](https://github.com/anycable/anycable/blob/master/spec/support/test_factory.rb) for example.
