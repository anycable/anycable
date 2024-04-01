# JWT authentication

AnyCable provides support for [JWT][jwt]-based authentication and identification.

We use the term "identification", because you can also pass a properly structured information as a part of the token to not only authentication the connection but also set up _identifiers_ (in terms of Action Cable). This approach brings the following benefits:

- **Performance**. No RPC call is required during the connection initiation, since we already have identification information. Thus, less load on the RPC server, much faster connection time (at least, 2x faster).
- **Usability**. Universal way of dealing with credentials (no need to deal with cookies for web and whatever else for mobile apps).
- **Security**. CSRF-safe by design. Configurable life time for tokens makes it easier to keep access under control.

## Usage

> See the [demo](https://github.com/anycable/anycable_rails_demo/pull/23) of using JWT identification in a Rails app with [AnyCable JS client library][anycable-client].

**NOTE**: Currently, we only support the HMAC signing algorithms.

By default, the `--secret` configuration parameter is used as a JWT secret key. If you want to use a custom key for JWT, you can specify it via the `--jwt_secret` (`ANYCABLE_JWT_SECRET`) parameter.

Other configuration options are:

- (_Optional_) **--jwt_param** (`ANYCABLE_ID_PARAM`, default: "jid"): the name of a query string param or an HTTP header, which carries a token. The header name is prefixed with `X-`.
- (_Optional_) **--enforce_jwt** (`ANYCABLE_ENFORCE_JWT`, default: false): whether to require all connection requests to contain a token. Connections without a token would be rejected right away. If not set, the servers fallbacks to the RPC call (if RPC is configured) or would be accepted if authentication is disabled (`--noauth`).

A client must provide an identification token either via a query param or via an HTTP header (if possible). For example:

```js
import { createCable } from '@anycable/web'

let cable = createCable('ws://cable.example.com/cable?jid=[JWT_TOKEN]')
```

The token MUST include the `ext` claim with the JSON-encoded connection identifiers.

## Generating tokens

### Rails/Ruby

When using AnyCable Ruby/Rails SDK, you can generate tokens as follows:

```ruby
token = AnyCable::JWT.encode({user: current_user})

# Setting TTL is also possible
token = AnyCable::JWT.encode({user: current_user}, expires_at: 10.minutes.from_now)
```

If you don't want to use our SDKs (why?), here is how you can generate tokens yourself:

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

## JavaScript/TypeScript

You can use [AnyCable server-side JS SDK](https://github.com/anycable/anycable-serverless-js) to generate tokens as follows:

```js
import { identificator } from "@anycable/serverless-js";

const jwtSecret = "very-secret";
const jwtTTL = "1h";

export const identifier = identificator(jwtSecret, jwtTTL);

// Then, somewhere in your code, generate a token and provide it to the client
const userId = authenticatedUser.id;
const token = await identifier.generateToken({ userId });
```

## PHP

You can use the following snippet to generate tokens in PHP:

```php
use Firebase\JWT\JWT;

$identifiers = ['user_id' => 42];
$payload = ['ext' => json_encode($identifiers), 'exp' => time() + 300];
$jwt = JWT::encode($payload, $ENCRYPTION_KEY, 'HS256');
```

## Python

Here is an example Python code to generate AnyCable tokens:

```python
import json
import jwt
import time

identifiers = {'user_id': 42}
payload = {'ext': json.dumps(identifiers), 'exp': int(time.time()) + 300}
jwt.encode(payload, ENCRYPTION_KEY, algorithm='HS256')
```

### Handling expired tokens

> 🎥 Check out this [AnyCasts episode](https://anycable.io/blog/anycasts-using-anycable-client/) to learn more about the expiration problem and how to solve it using [anycable-client](https://github.com/anycable/anycable-client).

Whenever a server encounters a token that has expired, it rejects the connection and send the `disconnect` message with `reason: "token_expired"`. It's a client responsibility to handle this situation and refresh the token.

See, for example, how [anycable-client handles this](https://github.com/anycable/anycable-client#refreshing-authentication-tokens).

[jwt]: https://jwt.io
[anycable-client]: https://github.com/anycable/anycable-client
