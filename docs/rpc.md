# AnyCable RPC

AnyCable allows you to control all the real-time communication logic from your backend application. For that, AnyCable uses a _remote procedure call_ (RPC) mechanism to delegate handling of connection lifecycle events and processing of incoming messages (subscriptions, arbitrary actions).

Using RPC is required if you design your real-time logic using _Channels_ (like in Rails Action Cable). For primitive pub/sub, you can run AnyCable in a [standalone mode](./getting_started.md#standalone-mode-pubsub-only), i.e., without RPC.

## RPC over gRPC

AnyCable is built for performance. Hence, it defaults to gRPC as a transport/protocol for RPC communication.

By default, AnyCable tries to connect to a gRPC server at `localhost:50051`:

```sh
$ anycable-go

2024-03-06 14:09:23.532 INF Starting AnyCable 1.5.0-4f16b99 (with mruby 1.2.0 (2015-11-17)) (pid: 21540, open file limit: 122880, gomaxprocs: 8) nodeid=6VV3mO
...
2024-03-06 14:09:23.533 INF RPC controller initialized: localhost:50051 (concurrency: 28, impl: grpc, enable_tls: false, proto_versions: v1) nodeid=6VV3mO context=rpc
```

You can change this setting by providing `--rpc_host` option or `ANYCABLE_RPC_HOST` env variable.

[AnyCable Ruby][anycable-ruby] library comes with AnyCable gRPC server out-of-the-box.

For other platforms, you can use definitions for the AnyCable gRPC service ([rpc.proto][proto]) to write your custom RPC server.

## RPC over HTTP

AnyCable also supports RPC communication over HTTP. It's a good alternative if you don't want to deal with separate gRPC servers or you are using a platform that doesn't support gRPC (e.g., Heroku, Google Cloud Run).

To connect to an HTTP RPC server, you must specify the `--rpc_host` (or `ANYCABLE_RPC_HOST`) with the explicit `http://` (or `https://`) scheme:

```sh
$ anycable-go --rpc_host=http://localhost:3000/_anycable

2024-03-06 14:21:37.231 INF Starting AnyCable 1.5.0-4f16b99 (with mruby 1.2.0 (2015-11-17)) (pid: 26540, open file limit: 122880, gomaxprocs: 8) nodeid=VkaKtV
...
2024-03-06 14:21:37.232 INF RPC controller initialized: http://localhost:3000/_anycable (concurrency: 28, impl: http, enable_tls: false, proto_versions: v1) nodeid=VkaKtV context=rpc
```

[AnyCable Ruby][anycable-ruby] library allows you to mount AnyCable HTTP RPC right into your Rack-compatible web server.

[AnyCable JS][anycable-server-js] provides HTTP handlers for processing HTTP RPC requests.

For other platforms, check out our Open API specification with examples on how to implement AnyCable HTTP RPC endpoint yourself:  [anycable.spotlight.io](https://anycable.stoplight.io).

### Configuration and security

If HTTP RPC endpoint is open to public (which is usually the case, since HTTP RPC is often embedded into the main application web server), it MUST be protected from unauthorized access.

AnyCable can be configured to pass an authentication key along RPC requests in the `Authorization: Bearer <secret key>` header.

You can either configure the RPC server key explicitly via the `--http_rpc_secret` (or `ANYCABLE_HTTP_RPC_SECRET`) parameter or use the application secret (`--secret`) to generate one using the following formula (in Ruby):

```ruby
rpc_secret_key = OpenSSL::HMAC.hexdigest("SHA256", "<APPLICATION SECRET>", "rpc-cable")
```

Alternatively, using `openssl`:

```sh
echo -n 'rpc-cable' | openssl dgst -sha256 -hmac '<your secret>' | awk '{print $2}'
```

If you use official AnyCable libraries at the RPC server side, you don't need to worry about these details yourself (the shared application secret is used to generate tokens at both sides). Just make sure both sides share the same application or HTTP RPC secret.

Other available configuration options:

- `http_rpc_timeout`: timeout for HTTP RPC requests (default: 3s).

## Concurrency settings

AnyCable uses a single Go gRPC client\* to communicate with AnyCable RPC servers (see [the corresponding PR](https://github.com/anycable/anycable-go/pull/88)). We limit the number of concurrent RPC calls to avoid flooding servers (and getting `ResourceExhausted` exceptions in response).

\* A single _client_ doesn't necessary mean a single connection; a Go gRPC client could maintain multiple HTTP2 connections, for example, when using [DNS-based load balancing](../deployment/load_balancing).

We limit the number of concurrent RPC calls at the application level (to prevent RPC servers overload). By default, the concurrency limit is equal to **28**, which is intentionally less than the default RPC pool size of **30** (for example, in Ruby gRPC server implementation): there is a tiny lag between the times when the response is received by the client and the corresponding worker is returned to the pool. Thus, whenever you update the concurrency settings, make sure that the AnyCable value is _slightly less_ than one we use by default for AnyCable Ruby gRPC server.

You can change this value via `--rpc_concurrency` (`ANYCABLE_RPC_CONCURRENCY`) parameter.

### Adaptive concurrency

<p class="pro-badge-header"></p>

AnyCable Pro provides the **adaptive concurrency** feature. When it is enabled, AnyCable automatically adjusts its RPC concurrency limit depending on the two factors: the number of `ResourceExhausted` errors (indicating that the current concurrency limit is greater than RPC servers capacity) and the number of pending RPC calls (indicating the current concurrency is too small to process incoming messages). The first factor (exhausted errors) has a priority (so if we have both a huge backlog and a large number of errors we decrease the concurrency limit).

You can enable the adaptive concurrency by specifying 0 as the `--rpc_concurrency` value:

```sh
$ anycable-go --rpc_concurrency=0

...

2024-03-06 14:21:37.232 INF RPC controller initialized: \
  localhost:50051 (concurrency: auto (initial=25, min=5, max=100), enable_tls: false, proto_versions: v1) \
  nodeid=VkaKtV context=rpc
```

You should see the `(concurrency: auto (...))` in the logs. You can also specify the upper and lower bounds for concurrency via the following parameters:

```sh
$ anycable-go \
  --rpc_concurrency=0 \
  --rpc_concurrency_initial=30 \
  --rpc_concurrency_max=50 \
  --rpc_concurrency_min=5
```

You can also monitor the current concurrency value via the `rpc_capacity_num` metrics. Read more about [AnyCable instrumentation](./instrumentation.md).

[proto]: ../misc/rpc_proto.md
[anycable-ruby]: https://github.com/anycable/anycable
[anycable-server-js]: https://github.com/anycable/anycable-serverless-js
