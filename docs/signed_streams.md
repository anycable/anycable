# Signed streams (Hotwire, CableReady) <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' />

AnyCable PRO provides an ability to terminate Hotwire ([Turbo Streams](https://turbo.hotwired.dev/handbook/streams)) and [CableReady](https://cableready.stimulusreflex.com) (v5+) subscriptions at the WS server without performing RPC calls. Thus, you can make subscriptions blazingly fast and reduce the load on the RPC server.

In combination with [JWT identification](./jwt_identification.md), this feature makes it possible to avoid running RPC server at all in case you only need Hotwire/CableReady functionality.

## Usage with Hotwire / Turbo Streams

We assume that you use an Action Cable integration provided by the [turbo-rails][] gem.

Whenever you use the `#turbo_stream_from` helper, Rails generates a _signed stream name_ to pass along the subscription request.
In order to _verify_ it at the AnyCable Go side, we need to know the encryption key, so it must be the same for the Rails app and for the AnyCable server.

Specify the verifier key in the Rails config:

```ruby
# config/environments/production.rb
config.turbo.signed_stream_verifier_key = "s3cЯeT"
```

For AnyCable Go, you must specify the same key as the value for the `turbo_rails_key` configuration option (`ANYCABLE_TURBO_RAILS_KEY` env var) to activate the fast lane:

```sh
anycable-go --turbo_rails_key=s3cЯeT

# or
ANYCABLE_TURBO_RAILS_KEY=s3cЯeT anycable-go
```

You should the following line in the logs at the server start:

```sh
...
INFO 2021-09-14T12:49:34.274Z context=main Using channels router: Turbo::StreamsChannel
...
```

## Usage with CableReady

**NOTE:** This feature requires upcoming CableReady v5. Currently, [preview versions](https://rubygems.org/gems/cable_ready) are available.

Whenever you use the `#stream_from` helper, CableReady generates a _signed stream identifier_ to pass along the subscription request.

In order to _verify_ it at the AnyCable Go side, we need to know the encryption key, so it must be the same for the Rails app and for the AnyCable server.

Specify the verifier key in the CableReady config:

```ruby
# config/initializers/cable_ready.rb
CableReady.configure do |config|
  config.verifier_key = "s3cЯeT"
end
```

For AnyCable Go, you must specify the same key as the value for the `cable_ready_key` configuration option (`ANYCABLE_CABLE_READY_KEY` env var) to activate the fast lane:

```sh
anycable-go --cable_ready_key=s3cЯeT

# or
ANYCABLE_CABLE_READY_KEY=s3cЯeT anycable-go
```

You should the following line in the logs at the server start:

```sh
...
INFO 2021-09-14T12:52:59.371Z context=main Using channels router: CableReady::Stream
...
```

[turbo-rails]: https://github.com/hotwired/turbo-rails
