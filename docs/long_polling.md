# Long polling support

<p class="pro-badge-header"></p>

AnyCable Pro supports alternative transport protocols, such as long polling. Even though WebSockets are widely supported, they still can be blocked by corporate firewalls and proxies. Long polling is a simplest alternative for such cases, especially if you want to support legacy browsers or clients without official client SDKs.

**IMPORTANT:** Long-polling sessions are not distributed by design (at least for now). For AnyCable-Go clusters, **sticky sessions must be used** for polling connections.

## Usage

First, you need to enable long polling support in `anycable-go`:

```sh
$ anycable-go --poll

  INFO 2023-06-29T03:44:22.460Z context=main Starting AnyCable 1.4.0-pro-72d0c60 (with mruby 1.2.0 (2015-11-17)) (pid: 34235, open file limit: 122880, gomaxprocs: 8, netpoll: true)
  ...
  INFO 2023-06-29T03:44:22.462Z context=main Handle long polling requests at http://0.0.0.0:8080/lp (poll_interval: 15, keepalive_timeout: 5)
```

Now you can use the `/lp` endpoint to establish a long polling connection. Let's see how we can do that at the client side.

### Using with AnyCable JS SDK

[AnyCable JS client][anycable-client] provides long polling support by the means of the [@anycable/long-polling][] plugin. You can use it as follows:

```js
import { createCable } from '@anycable/web'
import { LongPollingTransport } from '@anycable/long-polling'

// Create a transport object and pass the URL to the AnyCable server's long polling endpoint
const lp = new LongPollingTransport('http://my.anycable.host/lp')

// Pass the transport to the createCable or createConsumer function via the `fallbacks` option
export default createCable({fallbacks: [lp]})
```

That's it! Now your client will fallback to long polling if WebSocket connection can't be established.

See full documentation [here][@anycable/long-polling]

### Using with a custom client

You can use any HTTP client to communicate with AnyCable via a long polling endpoint.

#### Establishing a connection

To establish a connection, client MUST send a `POST` request to the `/lp` endpoint. The authentication is performed based on the request data (cookies, headers, etc.), i.e., similar to WebSocket connections.

Client MAY send commands along with the initial request. The commands are processed by server only if authentication is successful. See below for the commands format.

If authentication is successful, the server MUST respond with a 20x status code and a unique poll session identifier in the `X-Anycable-Poll-ID` response header. The response body MAY include messages for the client.

If authentication is unsuccessful, the server MUST respond with a 401 status code. The response body MAY include messages for the client.

All other status codes are considered as errors.

#### Polling

Client MUST send a `POST` request to the `/lp` endpoint with the `X-Anycable-Poll-ID` header set to the poll session identifier received during the initial connection to receive messages from the server.

Client MAY send commands along with the poll request.

#### Stale session

If client doesn't send a poll request for a certain period of time (see `--poll_keepalive_timeout` option below), the server MUST close the poll session and respond with a 401 status code to the next polling request with the session's ID. Server MAY send a `session_expired` disconnect message to the client.

#### Communication format

Both client and server MUST use JSONL (JSON Lines) format for communication. JSONL is a sequence of JSON objects separated by newlines (`\n`). The last object MUST be followed by a newline.

For example, client MAY send the following commands along the initial request:

```json
{"command":"subscribe", "identifier":"chat_1"}
{"command":"subscribe","identifier":"presence_1"}
```

Server MAY respond with the following messages in the response body:

```json
{"type":"welcome"}
{"type":"confirm_subscription","identifier":"chat_1"}
{"type":"confirm_subscription","identifier":"presence_1"}
```

## Configuration

The following options are available:

- `--poll_path` (`ANYCABLE_POLL_PATH`) (default: `/lp`): a long polling endpoint path.
- `--poll_interval` (`ANYCABLE_POLL_INTERVAL`) (default: 15): polling interval in seconds.
- `--poll_flush_interval` (`ANYCABLE_POLL_FLUSH_INTERVAL`) (default: 500): defines for how long to buffer server-to-client messages before flushing them to the client (in milliseconds).
- `--poll_max_request_size` (`ANYCABLE_POLL_MAX_REQEUEST_SIZE`) (default: 64kB): maximum acceptible request body size (in bytes).
- `--poll_keepalive_timeout` (`ANYCABLE_POLL_KEEPALIVE_TIMEOUT`) (default: 5): defines for how long to keep a poll session alive between requests (in seconds).

## CORS

Server responds with the `Access-Control-Allow-Origin` header set to the value of the `--allowed_origins` option (default: `*`). If you want to restrict the list of allowed origins, you can pass a comma-separated list of domains to the option. See [documentation](./configuration.md).

## Instrumentation

When long polling enabled, the following metrics are available:

- `long_poll_clients_num`: number of active long polling clients.
- `long_poll_stale_requests_total`: number of stale requests (i.e., requests that were sent after the poll session was closed).

[anycable-client]: https://github.com/anycable/anycable-client
[@anycable/long-polling]: https://github.com/anycable/anycable-client/tree/master/packages/long-polling
