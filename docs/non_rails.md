# Using AnyCable Ruby without Rails

AnyCable Ruby can be used without Rails, thus, allowing you to bring real-time and Action Cable-like functionality into your Ruby application.

## Installation

Add `anycable` gem to your `Gemfile`:

```ruby
gem "anycable", "~> 1.1"

# when using Redis-backed broadcast adapter
gem "redis", ">= 4.0"

# when using NATS-backed broadcast adapter
gem "nats-pure", "~> 2"
```

If you don't plan to use _channels_ (RPC), you can go with the `anycable-core` gem instead of `anycable`.

## Pub/Sub only mode

To use AnyCable in a standalone (pub/sub only) mode (see [docs](../anycable-go/getting_started.md)), you need to use the following APIs:

- Broadcasting.

  See the [corresponding documentation](./broadcast_adapters.md).

- JWT authentication.

  You can generate AnyCable JWT tokens as follows:

  ```ruby
  # Client entitity identifiers (usually, user ID or similar)
  identifiers = {user_id: User.first.id}

  token = AnyCable::JWT.encode(identifiers)
  ```

- Signed streams.

  You the following API to sign streams:

  ```ruby
  signed_name = AnyCable.signed_stream("chat/42")
  ```

**NOTE:** For JWT authentication and signed streams, the application secret or dedicated secrets MUST be provided. The values MUST match the ones configured at the AnyCable server side.

## Using channels via Lite Cable

There is a ready-to-go framework – [Lite Cable](https://github.com/palkan/litecable) – which can be used for application logic. It also supports AnyCable out-of-the-box.

> Learn how to use AnyCable with Hanami in the ["AnyCable off Rails: connecting Twilio streams with Hanami"](https://evilmartians.com/chronicles/anycable-goes-off-rails-connecting-twilio-streams-with-hanami) blog post.

## Custom channels implementation

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
