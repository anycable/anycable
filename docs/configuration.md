# AnyCable-Go configuration

You can configure AnyCable-Go via CLI options, e.g.:

```sh
$ anycable-go --rpc_host=localhost:50051 --headers=cookie \
              --redis_url=redis://localhost:6379/5 --redis_channel=__anycable__ \
              --host=localhost --port=8080
```

Or via the corresponding environment variables (i.e. `ANYCABLE_RPC_HOST`, `ANYCABLE_REDIS_URL`, etc.).

## Primary settings

Here is the list of the most commonly used configuration parameters.

**NOTE:** To see all available options run `anycable-go -h`.

---

**--host**, **--port** (`ANYCABLE_HOST`, `ANYCABLE_PORT` or `PORT`)

Server host and port (default: `"localhost:8080"`).

**--path** (`ANYCABLE_PATH`)

WebSocket endpoint path (default: `"/cable"`).

You can specify multiple paths separated by commas.

You can also use wildcards (at the end of the paths) or path placeholders:

```sh
anycable-go --path="/cable,/admin/cable/*,/accounts/{tenant}/cable"
```

**--allowed_origins** (`ANYCABLE_ALLOWED_ORIGINS`)

Comma-separated list of hostnames to check the Origin header against during the WebSocket Upgrade.
Supports wildcards, e.g., `--allowed_origins=*.evilmartians.io,www.evilmartians.com`.

**--broadcast_adapter** (`ANYCABLE_BROADCAST_ADAPTER`, default: `redis`)

[Broadcasting adapter](./broadcasting.md) to use. Available options: `redis` (default), `redisx`, `nats`, and `http`.

When HTTP adapter is used, AnyCable-Go accepts broadcasting requests on `:8090/_broadcast`.

You can also enable multiple adapters at once by specifying them separated by commas.

**--broker** (`ANYCABLE_BROKER`, default: `none`)

[Broker](./broker.md) adapter to use.

**--pubsub** (`ANYCABLE_PUBSUB`, default: `none`)

Pub/Sub adapter to use to distribute broadcasted messages within the cluster (when non-distributed broadcasting adapter is used). **Required for broker**.

**--streams_secret** (`ANYCABLE_STREAMS_SECRET`)

A secret key used to verify [signed_streams](./signed_streams.md). If not set, the `--secret` setting is used (see below).

## RPC settings

**--rpc_host** (`ANYCABLE_RPC_HOST`)

RPC service address (default: `"localhost:50051"`). You can also specify the scheme part to indicate which RPC protocol to use, gRPC or HTTP (gRPC is assumed by default). See below for more details on [HTTP RPC](./rpc.md#rpc-over-http).

**--norpc** (`ANYCABLE_NORPC=true`)

This setting disables the RPC component completely. That means, you can only use AnyCable in a standalone mode (with [JWT authentication](./jwt_identification.md) and [signed streams](./signed_streams.md)).

**--headers** (`ANYCABLE_HEADERS`)

Comma-separated list of headers to proxy to RPC (default: `"cookie"`).

**--proxy-cookies** (`ANYCABLE_PROXY_COOKIES`)

Comma-separated list of cookies to proxy to RPC (default: all cookies).

## Security/access settings

**--secret** (`ANYCABLE_SECRET`)

A common secret key used by the following components (unless a specific key is specified): [JWT authentication](./jwt_identification.md), [signed streams](./signed_streams.md).

**--broadcast_key** (`ANYCABLE_BROADCAST_KEY`)

A secret key used to authenticate broadcast requests. See [broadcasting docs](./broadcasting.md). You can use the special "none" value to disable broadcasting authentication.

**--noauth** (`ANYCABLE_NOAUTH=true`)

This setting disables client authentication checks (so, anyone is allowed to connect). Use it with caution. **NOTE**: if you use _enforced_ JWT authentication, the `--noauth` option has no effect.

**--public_streams** (`ANYCABLE_PUBLIC_STREAMS=true`)

Setting this value allows direct subscribing to streams using unsigned names (see more in the [signed streams docs](./signed_streams.md)).

**--public** (`ANYCABLE_PUBLIC=true`)

This is a shortcut to specify both `--noauth`, `--public_streams` and `--broadcast_key=none`, so you can use AnyCable without any protection. **Do not do this in production**.

## HTTP API

**--http_broadcast_port** (`ANYCABLE_HTTP_BROADCAST_PORT`, default: `8090`)

You can specify on which port to receive broadcasting requests (NOTE: it could be the same port as the main HTTP server listens to).

## Redis configuration

**--redis_url** (`ANYCABLE_REDIS_URL` or `REDIS_URL`)

Redis URL to connect to (default: `"redis://localhost:6379/5"`). Used by the corresponding pub/sub, broadcasting, and broker adapters.

**--redis_channel** (`ANYCABLE_REDIS_CHANNEL`)

Redis channel for broadcasting (default: `"__anycable__"`). When using the `redisx` adapter, it's used as a name of the Redis stream.

**--redis_disable_cache** (`ANYCABLE_REDIS_DISABLE_CACHE`)

Disable [`CLIENT TRACKING`](https://redis.io/commands/client-tracking/) (it could be blocked by some managed Redis providers).

## NATS configuration

**--nats_servers** (`ANYCABLE_NATS_SERVERS`)

The list of [NATS][] servers to connect to (default: `"nats://localhost:4222"`). Used by the corresponding pub/sub, broadcasting, and broker adapters.

**--nats_channel** (`ANYCABLE_NATS_CHANNEL`)

NATS channel for broadcasting (default: `"__anycable__"`).

## Logging settings

**--log_level** (`ANYCABLE_LOG_LEVEL`)

Logging level (default: `"info"`).

**--debug** (`ANYCABLE_DEBUG`)

Enable debug mode (more verbose logging).

## Presets

AnyCable-Go comes with a few built-in configuration presets for particular deployments environments, such as Heroku or Fly. The presets are detected and activated automatically. As an indication, you can find a line in the logs:

```sh
INFO ... context=config Loaded presets: fly
```

To disable automatic presets activation, provide `ANYCABLE_PRESETS=none` environment variable (or pass the corresponding option to the CLI: `anycable-go --presets=none`).

**NOTE:** Presets do not override explicitly provided configuration values.

### Preset: fly

Automatically activated if all of the following environment variables are defined: `FLY_APP_NAME`, `FLY_REGION`, `FLY_ALLOC_ID`.

The preset provide the following defaults:

- `host`: "0.0.0.0"
- `http_broadcast_port`: `$PORT` (set to the same value as the main HTTP port).
- `broadcast_adapter`: "http" (unless Redis is configured)
- `enats_server_addr`: "nats://0.0.0.0:4222"
- `enats_cluster_addr`: "nats://0.0.0.0:5222"
- `enats_cluster_name`: "\<FLY_APP_NAME\>-\<FLY_REGION\>-cluster"
- `enats_cluster_routes`: "nats://\<FLY_REGION\>.\<FLY_APP_NAME\>.internal:5222"
- `enats_gateway_advertise`: "\<FLY_REGION\>.\<FLY_APP_NAME\>.internal:7222" (**NOTE:** You must set `ANYCABLE_ENATS_GATEWAY` to `nats://0.0.0.0:7222` and configure at least one gateway address manually to enable gateways).

Also, [embedded NATS](./embedded_nats.md) is enabled automatically if no other pub/sub adapter neither Redis is configured. Similarly, pub/sub, broker and broadcast adapters using embedded NATS are configured automatically, too. Thus, by default, AnyCable-Go setups a NATS cluster automatically (within a single region), no configuration is required.

If the `ANYCABLE_FLY_RPC_APP_NAME` env variable is provided, the following defaults are configured as well:

- `rpc_host`: "dns:///\<FLY_REGION\>.\<ANYCABLE_FLY_RPC_APP_NAME\>.internal:50051"

### Preset: heroku

Automatically activated if all of the following environment variables are defined: `HEROKU_DYNO_ID`, `HEROKU_APP_ID`. **NOTE:** These env vars are defined only if the [Dyno Metadata feature](https://devcenter.heroku.com/articles/dyno-metadata) is enabled.

The preset provides the following defaults:

- `host`: "0.0.0.0".
- `http_broadcast_port`: `$PORT` (to make HTTP endpoint accessible from other applications).

## Per-client settings

A client MAY override default values for the settings listed below by providing the corresponding parameters in the WebSocket URL query string:

- `?pi=<seconds>`: ping interval (overrides `--ping_interval`).
- `?ptp=<s | ms | ns>`: ping timestamp precision (overrides `--ping_timestamp_precision`).

For example, using the following URL, you can set the ping interval to 10 seconds and the timestamp precision to milliseconds:

```txt
ws://localhost:8080/cable?pi=10&ptp=ms
```

## TLS

To secure your `anycable-go` server provide the paths to SSL certificate and private key:

```sh
anycable-go --port=443 -ssl_cert=path/to/ssl.cert -ssl_key=path/to/ssl.key

=> INFO time context=http Starting HTTPS server at 0.0.0.0:443
```

If your RPC server requires TLS you can enable it via `--rpc_enable_tls` (`ANYCABLE_RPC_ENABLE_TLS`).

If RPC server uses certificate issued by private CA, then you can pass either its file path or PEM contents with `--rpc_tls_root_ca` (`ANYCABLE_RPC_TLS_ROOT_CA`).

If RPC uses self-signed certificate, you can disable RPC server certificate verification by setting `--rpc_tls_verify` (`ANYCABLE_RPC_TLS_VERIFY`) to `false`, but this is insecure, use only in test/development.

## Disconnect settings

AnyCable-Go notifies an RPC server about disconnected clients asynchronously with a rate limit. We do that to allow other RPC calls to have higher priority (because _live_ clients are usually more important) and to avoid load spikes during mass disconnects (i.e., when a server restarts).

That could lead to the situation when the _disconnect queue_ is overwhelmed, and we cannot perform all the `Disconnect` calls during server shutdown. Thus, **RPC server may not receive all the disconnection events** (i.e., `disconnect` and `unsubscribed` callbacks in your code).

If you rely on `disconnect` callbacks in your code, you can tune the default disconnect queue settings to provide better guarantees\*:

**--disconnect_rate** (`ANYCABLE_DISCONNECT_RATE`)

The max number of `Disconnect` calls per-second (default: 100).

**--disconnect_timeout** (`ANYCABLE_DISCONNECT_TIMEOUT`)

The number of seconds to wait before forcefully shutting down a disconnect queue during the server graceful shutdown (default: 5).

Thus, the default configuration can handle a backlog of up to 500 calls. By increasing both values, you can reduce the number of lost disconnect notifications.

**--disconnect_mode** (`ANYCABLE_DISCONNECT_MODE`)

This parameter defines when a Disconnect call should be made for a session. The default is "auto", which means that the Disconnect call is made only if we detected the client _interest_ in it. Currently, we only skip Disconnect calls for sessions authenticated via [JWT](./jwt_identification.md) and using [signed streams](./signed_streams.md) (Hotwire or CableReady).

Other available modes are "always" and "never". Thus, to disable Disconnect call completely, use `--disconnect_mode=never`.

Using `--disconnect_mode=always` is useful when you have some logic in the `ApplicationCable::Connetion#disconnect` method and you want to invoke it even for JWT and signed streams sessions.

**NOTE:** AnyCable tries to make a Disconnect call for active sessions during the server shutdown. However, if the server is killed with `kill -9` or crashes, the disconnect queue is not flushed, and some disconnect events may be lost. If you experience higher queue sizes during deployments, consider increasing the shutdown timeout by tuning the `--shutdown_timeout` parameter.

### Slow drain mode

<p class="pro-badge-header"></p>

AnyCable-Go PRO provides the **slow drain** mode for disconnecting clients during shutdown. When it is enabled, AnyCable do not try to disconnect all active clients as soon as a shutdown signal is received. Instead, spread the disconnects over the graceful shutdown period. This way, you can reduce the load on AnyCable servers during deployments (i.e., avoid the _thundering herd_ situation).

You can enable this feature by providing the `--shutdown_slowdrain` option or setting the `ANYCABLE_SHUTDOWN_SLOWDRAIN` environment variable to `true`. You should see the following log message on shutdown indicating that the slow drain mode is enabled:

```sh
INFO 2023-08-04T07:16:14.339Z context=node Draining 1234 active connections slowly for 24.7s
```

The actual _drain period_ is slightly less than the shutdown timeout—we need to reserve some time to complete RPC calls. Also, there is a maximum interval between disconnects (500ms), so we don't wait too long when the number of clients is not that big.

## GOMAXPROCS

We use [automaxprocs][] to automatically set the number of OS threads to match Linux container CPU quota in a virtualized environment, not a number of _visible_ CPUs (which is usually much higher).

This feature is enabled by default. You can opt-out by setting `GOMAXPROCS=0` (in this case, the default Go mechanism of defining the number of threads is used).

You can find the actual value for GOMAXPROCS in the starting logs:

```sh
INFO 2022-06-30T03:31:21.848Z context=main Starting AnyCable 1.2.0-c4f1c6e (with mruby 1.2.0 (2015-11-17)) (pid: 39705, open file limit: 524288, gomaxprocs: 8)
```

[automaxprocs]: https://github.com/uber-go/automaxprocs
[NATS]: https://nats.io
