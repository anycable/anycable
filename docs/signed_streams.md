# Signed streams

AnyCable allows you to subscribe to _streams_ without using _channels_ (in Action Cable terminology). Channels is a great way to encapsulate business-logic for a given real-time feature, but in many cases all we need is a good old explicit pub/sub. That's where the **signed streams** feature comes into play.

> You read more about the Action Cable abstract design, how it compares to direct pub/sub and what are the pros and cons from this [Any Cables Monthly issue](https://anycable.substack.com/p/any-cables-monthly-18). Don't forget to subscribe!

Signed streams work as follows:

- Given a stream name, say, "chat/2024", you generate its signed version using a **secret key** (see below on the signing algorithm)

- On the client side, you subscribe to the "$pubsub" channel and provide the signed stream name as a `signed_stream_name` parameter

- AnyCable process the subscribe command, verifies the stream name and completes the subscription (if verified).

For verification, you MUST provide the **secret key** via the `--streams_secret` (`ANYCABLE_STREAMS_SECRET`) parameter for AnyCable.

## Full-stack example: Rails

Let's consider an example of using signed stream in a Rails application.

Assume that we want to subscribe a user with ID=17 to their personal notifications channel, "notifications/17".

First, we need to generate a signed stream name:

```ruby
signed_name = AnyCable::Streams.signed("notifications/17")
```

Or you can use the `#signed_stream_name` helper in your views

```erb
<div
  data-controller="notifications"
  data-notifications-stream="<%= signed_stream_name("notifications/#{current_user.id}") %>">

</div>
```

By default, AnyCable uses `Rails.application.secret_key_base` to sign streams. We recommend configuring a custom secret though (so you can easily rotate values at both ends, the Rails app and AnyCable servers). You can specify it via the `streams_secret` configuration parameter (in `anycable.yml`, credentials, or environment).

Then, on the client side, you can subscribe to this stream as follows:

```js
// using @rails/actioncable
let subscription = consumer.subscriptions.create(
  {channel: "$pubsub", signed_stream_name: stream},
  {
    received: (msg) => {
      // handle notification msg
    }
  }
)

// using @anycable/web
let channel = cable.streamFromSigned(stream);
channel.on("message", (msg) => {
  // handle notification
})
```

Now you can broadcast messages to this stream as usual:

```ruby
ActionCable.server.broadcast "notifications/#{user.id}", payload
```

## Public (unsigned) streams

Sometimes you may want to skip all the signing ceremony and use plain stream names instead. With AnyCable, you can do that by enabling the `--public_streams` option (or `ANYCABLE_PUBLIC_STREAMS=true`) for the AnyCable server:

```sh
$ anycable-go --public_streams

# or
$ ANYCABLE_PUBLIC_STREAMS=true anycable-go
```

With public streams enabled, you can subscribe to them as follows:

```js
// using @rails/actioncable
let subscription = consumer.subscriptions.create(
  {channel: "$pubsub", stream_name: "notifications/17"},
  {
    received: (msg) => {
      // handle notification msg
    }
  }
)

// using @anycable/web
let channel = cable.streamFrom("notifications/17");
channel.on("message", (msg) => {
  // handle notification
})
```

## Signing algorithm

We use the same algorithm as Rails uses in its [MessageVerifier](https://api.rubyonrails.org/v7.1.3/classes/ActiveSupport/MessageVerifier.html):

1. Encode the stream name by first converting it into a JSON string and then encoding in Base64 format.
1. Calculate a HMAC digest using the SHA256 hash function from the secret and the encoded stream name.
1. Concatenate the encoded stream name, a double dash (`--`), and the digest.

Here is the Ruby version of the algorithm:

```ruby
encoded = ::Base64.strict_encode64(JSON.dump(stream_name))
digest = OpenSSL::HMAC.hexdigest("SHA256", SECRET_KEY, encoded)
signed_stream_name = "#{encoded}--#{digest}"
```

The JavaScript (Node.js) version:

```js
import { createHmac } from 'crypto';

const encoded = Buffer.from(JSON.stringify(stream_name)).toString('base64');
const digest = createHmac('sha256', SECRET_KEY).update(encoded).digest('hex');
const signedStreamName = `${encoded}--${digest}`;
```

The Python version looks as follows:

```python
import base64
import json
import hmac
import hashlib

encoded = base64.b64encode(json.dumps(stream_name).encode('utf-8')).decode('utf-8')
digest = hmac.new(SECRET_KEY.encode('utf-8'), encoded.encode('utf-8'), hashlib.sha256).hexdigest()
signed_stream_name = f"{encoded}--{digest}"
```

The PHP version is as follows:

```php
$encoded = base64_encode(json_encode($stream_name));
$digest = hash_hmac('sha256', $encoded, $SECRET_KEY);
$signed_stream_name = $encoded . '--' . $digest;
```

## Hotwire and CableReady support

AnyCable provides an ability to terminate Hotwire ([Turbo Streams](https://turbo.hotwired.dev/handbook/streams)) and [CableReady](https://cableready.stimulusreflex.com) (v5+) subscriptions at the WebSocker server using the same signed streams functionality under the hood (and, thus, without performing any RPC calls to authorize subscriptions).

In combination with [JWT authentication](./jwt_identification.md), this feature makes it possible to avoid run AnyCable in a standalone mode for Hotwire/CableReady applications.

> 🎥 Check out this [AnyCasts episode](https://anycable.io/blog/anycasts-rails-7-hotwire-and-anycable/) to learn how to use AnyCable with Hotwire Rails application in a RPC-less way.

You must explicitly enable Turbo Streams or CableReady signed streams support at the AnyCable server side by specifying the `--turbo_streams` (`ANYCABLE_TURBO_STREAMS=true`) or `--cable_ready_streams` (`ANYCABLE_CABLE_READY_STREAMS=true`) option respectively.

You must also provide the `--streams_secret` corresponding to the secret you use for Turbo/CableReady. You can configure them in your Rails application as follows:

```ruby
# Turbo configuration

# config/environments/production.rb
config.turbo.signed_stream_verifier_key = "<SECRET>"

# CableReady configuration

# config/initializers/cable_ready.rb
CableReady.configure do |config|
  config.verifier_key = "<SECRET>"
end
```

You can also specify custom secrets for Turbo Streams and CableReady via the `--turbo_streams_secret` and `--cable_ready_secret` parameters respectively.
