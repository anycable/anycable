# JWT identification <img class='pro-badge' src='https://docs.anycable.io/assets/pro.svg' alt='pro' />

AnyCable Pro provides support for [JWT][jwt]-based authentication and identification.

You can pass a properly structured token along the connection request to authorize the connection and set up _identifiers_ (in terms of Action Cable). This approach brings the following benefits:

- **Performance**. No RPC call is required during the connection initiation, since we already have identification information. Thus, less load on the RPC server, much faster connection time (at least, 2x faster).
- **Usability**. Universal way of dealing with credentials (no need to deal with cookies for web and whatever else for mobile apps).
- **Security**. CSRF-safe by design. Configurable life time for tokens makes it easier to keep access under control.

## Usage

> See the [demo](https://github.com/anycable/anycable_rails_demo/pull/23) of using JWT identification in a Rails app with [AnyCable JS client library][anycable-client].

**NOTE**: Currently, we only support the HMAC signing algorithms.

First, you must enable JWT identification support in `anycable-go` by configuring the following params:

- **--jwt_id_key** (`ANYCABLE_JWT_ID_KEY`): the encryption key used to verify tokens.
- (_Optional_) **--jwt_id_param** (`ANYCABLE_JWT_ID_PARAM`): the name of a query string param or an HTTP header, which carries a token. The header name is prefixed with `X-`. Default: `jid` (and the `X-JID` header correspondingly).
- (_Optional_) **--jwt_id_enforce** (`ANYCABLE_JWT_ID_ENFORCE`): whether to require all connection requests to contain a token. Connections without a token would be rejected right away. If not set, the servers fallbacks to the RPC call (as w/o JWT enabled). Default: false.

A client must provide an identification token either via a query param or via an HTTP header (if possible). For example:

```js
import { createCable } from '@anycable/web'

let cable = createCable('ws://cable.example.com/cable?jid=[JWT_TOKEN]')
```

The token MUST include the `ext` claim with the JSON-encoded connection identifiers.

## Generating tokens

### Rails integration

Use [anycable-rails-jwt][] gem to integration JWT identification into your Rails app.

### Custom implementation

Here is an example Ruby code to generate tokens:

```ruby
require "jwt"
require "json"

ENCRYPTION_KEY = "some-sercret-key"

# !!! Expiration is the responsibility of the token issuer
exp = Time.now.to_i + 30

# Provides the serialized values for identifiers (`identified_by` in Action Cable)
identifiers = {user_id: 42}

# JWT payload
payload = {ext: identifiers.to_json, exp: exp}

puts JWT.encode payload, ENCRYPTION_KEY, "HS256"
```

### Handling expired tokens

Whenever a server encounters a token that has expired, it rejects the connection and send the `disconnect` message with `reason: "token_expired"`. It's a client responsibility to handle this situation and refresh the token.

See, for example, how [anycable-client handles this](https://github.com/anycable/anycable-client#refreshing-authentication-tokens).

[jwt]: https://jwt.io
[anycable-rails-jwt]: https://github.com/anycable/anycable-rails-jwt
[anycable-client]: https://github.com/anycable/anycable-client
