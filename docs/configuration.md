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

**--rpc_host** (`ANYCABLE_RPC_HOST`)

RPC service address (default: `"localhost:50051"`).

**--path** (`ANYCABLE_PATH`)

WebSocket endpoint path (default: `"/cable"`).

You can specify multiple paths separated by commas.

You can also use wildcards (at the end of the paths) or path placeholders:

```sh
anycable-go --path="/cable,/admin/cable/*,/accounts/{tenant}/cable"
```

**--headers** (`ANYCABLE_HEADERS`)

Comma-separated list of headers to proxy to RPC (default: `"cookie"`).

**--proxy-cookies** (`ANYCABLE_PROXY_COOKIES`)

Comma-separated list of cookies to proxy to RPC (default: all cookies).

**--allowed_origins** (`ANYCABLE_ALLOWED_ORIGINS`)

Comma-separated list of hostnames to check the Origin header against during the WebSocket Upgrade.
Supports wildcards, e.g., `--allowed_origins=*.evilmartians.io,www.evilmartians.com`.

**--broadcast_adapter** (`ANYCABLE_BROADCAST_ADAPTER`, default: `redis`)

[Broadcasting adapter](../ruby/broadcast_adapters.md) to use. Available options: `redis` (default), `redisx`, `nats`, and `http`.

When HTTP adapter is used, AnyCable-Go accepts broadcasting requests on `:8090/_broadcast`.

You can also enable multiple adapters at once by specifying them separated by commas.

**--broker** (`ANYCABLE_BROKER`, default: `none`)

[Broker](./broker.md) adapter to use.

**--pubsub** (`ANYCABLE_PUBSUB`, default: `none`)

Pub/Sub adapter to use to distribute broadcasted messages within the cluster (when non-distributed broadcasting adapter is used). **Required for broker**.

**--http_broadcast_port** (`ANYCABLE_HTTP_BROADCAST_PORT`, default: `8090`)

You can specify on which port to receive broadcasting requests (NOTE: it could be the same port as the main HTTP server listens to).

**--http_broadcast_secret** (`ANYCABLE_HTTP_BROADCAST_SECRET`)

Authorization secret to protect the broadcasting endpoint (see [Ruby docs](../ruby/broadcast_adapters.md#securing-http-endpoint)).

**--redis_url** (`ANYCABLE_REDIS_URL` or `REDIS_URL`)

Redis URL for pub/sub (default: `"redis://localhost:6379/5"`).

**--redis_channel** (`ANYCABLE_REDIS_CHANNEL`)

Redis channel for broadcasting (default: `"__anycable__"`). When using the `redisx` adapter, it's used as a name of the Redis stream.

**--nats_servers** (`ANYCABLE_NATS_SERVERS`)

The list of [NATS][] servers to connect to (default: `"nats://localhost:4222"`).

**--nats_channel** (`ANYCABLE_NATS_CHANNEL`)

NATS channel for broadcasting (default: `"__anycable__"`).

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
- `enats_server_addr`: "nats://0.0.0.0:4222"
- `enats_cluster_addr`: "nats://0.0.0.0:5222"
- `enats_cluster_name`: "\<FLY_APP_NAME\>-\<FLY_REGION\>-cluster"
- `enats_cluster_routes`: "nats://\<FLY_REGION\>.\<FLY_APP_NAME\>.internal:5222"
- `enats_gateway_advertise`: "\<FLY_REGION\>.\<FLY_APP_NAME\>.internal:7222" (**NOTE:** You must set `ANYCABLE_ENATS_GATEWAY` to `nats://0.0.0.0:7222` and configure at least one gateway address manually to enable gateways).

If the `ANYCABLE_FLY_RPC_APP_NAME` env variable is provided, the following defaults are configured as well:

- `rpc_host`: "dns:///\<FLY_REGION\>.\<ANYCABLE_FLY_RPC_APP_NAME\>.internal:50051"

### Preset: heroku

Automatically activated if all of the following environment variables are defined: `HEROKU_DYNO_ID`, `HEROKU_APP_ID`. **NOTE:** These env vars are defined only if the [Dyno Metadata feature](https://devcenter.heroku.com/articles/dyno-metadata) is enabled.

The preset provides the following defaults:

- `host`: "0.0.0.0".
- `http_broadcast_port`: `$PORT` (to make HTTP endpoint accessible from other applications).

## TLS

To secure your `anycable-go` server provide the paths to SSL certificate and private key:

```sh
anycable-go --port=443 -ssl_cert=path/to/ssl.cert -ssl_key=path/to/ssl.key

=> INFO time context=http Starting HTTPS server at 0.0.0.0:443
```

If your RPC server requires TLS you can enable it via `--rpc_enable_tls` (`ANYCABLE_RPC_ENABLE_TLS`).

## Concurrency settings

AnyCable-Go uses a single Go gRPC client\* to communicate with AnyCable RPC servers (see [the corresponding PR](https://github.com/anycable/anycable-go/pull/88)). We limit the number of concurrent RPC calls to avoid flooding servers (and getting `ResourceExhausted` exceptions in response).

\* A single _client_ doesn't necessary mean a single connection; a Go gRPC client could maintain multiple HTTP2 connections, for example, when using [DNS-based load balancing](../deployment/load_balancing).

We limit the number of concurrent RPC calls at the application level (to prevent RPC servers overload). By default, the concurrency limit is equal to **28**, which is intentionally less than the default RPC size (see [Ruby configuration](../ruby/configuration.md#concurrency-settings)): there is a tiny lag between the times when the response is received by the client and the corresponding worker is returned to the pool. Thus, whenever you update the concurrency settings, make sure that the AnyCable-Go value is _slightly less_ than the AnyCable-RPC one.

You can change this value via `--rpc_concurrency` (`ANYCABLE_RPC_CONCURRENCY`) parameter.

## Adaptive concurrency

<p class="pro-badge-header"></p>

AnyCable-Go Pro provides the **adaptive concurrency** feature. When it is enabled, AnyCable-Go automatically adjusts its RPC concurrency limit depending on the two factors: the number of `ResourceExhausted` errors (indicating that the current concurrency limit is greater than RPC servers capacity) and the number of pending RPC calls (indicating the current concurrency is too small to process incoming messages). The first factor (exhausted errors) has a priority (so if we have both a huge backlog and a large number of errors we decrease the concurrency limit).

You can enable the adaptive concurrency by specifying 0 as the `--rpc_concurrency` value:

```sh
$ anycable-go --rpc_concurrency=0

...

INFO 2023-02-23T15:26:13.649Z context=rpc RPC controller initialized: \
  localhost:50051 (concurrency: auto (initial=25, min=5, max=100), enable_tls: false, proto_versions: v1)
```

You should see the `(concurrency: auto (...))` in the logs. You can also specify the upper and lower bounds for concurrency via the following parameters:

```sh
$ anycable-go \
  --rpc_concurrency=0 \
  --rpc_concurrency_initial=30 \
  --rpc_concurrency_max=50 \
  --rpc_concurrency_min=5
```

You can also monitor the current concurrency value via the `rpc_capacity_num` metrics.

## Disconnect events settings

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

\* It's (almost) impossible to guarantee that `disconnect` callbacks would be called for 100%. There is always a chance of a server crash or `kill -9` or something worse. Consider an alternative approach to tracking client states (see [example](https://github.com/anycable/anycable/issues/99#issuecomment-611998267)).

## GOMAXPROCS

We use [automaxprocs][] to automatically set the number of OS threads to match Linux container CPU quota in a virtualized environment, not a number of _visible_ CPUs (which is usually much higher).

This feature is enabled by default. You can opt-out by setting `GOMAXPROCS=0` (in this case, the default Go mechanism of defining the number of threads is used).

You can find the actual value for GOMAXPROCS in the starting logs:

```sh
INFO 2022-06-30T03:31:21.848Z context=main Starting AnyCable 1.2.0-c4f1c6e (with mruby 1.2.0 (2015-11-17)) (pid: 39705, open file limit: 524288, gomaxprocs: 8)
```

[automaxprocs]: https://github.com/uber-go/automaxprocs
[NATS]: https://nats.io
