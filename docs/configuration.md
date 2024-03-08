# Configuration

AnyCable Ruby uses [`anyway_config`](https://github.com/palkan/anyway_config) gem for configuration. Thus,  it is possible to set configuration parameters through environment vars, `config/anycable.yml` file, etc.

When running a gRPC server via our CLI, you can also pass configuration variables as follows:

```sh
$ bundle exec anycable --rpc-host 0.0.0.0:50120 \
                       --redis-channel my_redis_channel \
                       --log-level debug
```

**NOTE:** CLI options take precedence over parameters from other sources (files, env).

## Primary settings

**secret** (`ANYCABLE_SECRET`) (_@since v1.5.0_)

The application secret used to secure AnyCable features: signed streams, JWT authentication, etc. We recommend setting this value as a single AnyCable-related application secret and rely on libraries to glue pieces together.

**streams_secret** (`ANYCABLE_STREAMS_SECRET`) (_@since v1.5.0_)

A dedicated secret key used to [sign streams](../anycable-go/signed_streams.md). If none specified, the application secret is used.

**broadcast_adapter** (`ANYCABLE_BROADCAST_ADAPTER`)

Broadcasting adapter to use. Available options out-of-the-box: `redis` (default), `http` (will be default in v2), `nats`, `redisx`. For adapter specific options, see [broadcast adapters documentation](./broadcast_adapters.md).

**broadcast_key** (`ANYCABLE_BROADCAST_KEY`) (_@since v1.5.0_)

A secret key used to authorize broadcast requests. Currently, only used by the HTTP adapter. If not set, the value is inferred from the application secret. See the [broadcast adapters documentation](./broadcast_adapters.md).

## JWT settings

AnyCable supports [JWT authentication](../anycable-go/jwt_identification.md) out-of-the-box.

AnyCable Ruby provides an API for generating tokens relying on the following configuration parameters:

- **jwt_secret** (`ANYCABLE_JWT_SECRET`) (_@since v1.5.0_)

  The secret key used to sign JWT tokens. Optional (the application secret is used if no JWT secret specified)

- **jwt_ttl** (`ANYCABLE_JWT_TTL`) (_@since v1.5.0_)

  The time-to-live (TTL) for tokens in seconds. Default: 3600 (1 hour).

## Presets

AnyCable Ruby comes with a few built-in configuration presets for particular deployments environments, such as Fly. The presets are detected and activated automatically

To disable automatic presets activation, provide `ANYCABLE_PRESETS=none` environment variable (or pass the corresponding option to the CLI: `bundle exec anycable --presets=none`).

**NOTE:** Presets do not override explicitly provided configuration values.

### Preset: fly

Automatically activated if all of the following environment variables are defined: `FLY_APP_NAME`, `FLY_REGION`, `FLY_ALLOC_ID`.

The preset provide the following defaults:

- `rpc_host`: "0.0.0.0:50051"

If the `ANYCABLE_FLY_WS_APP_NAME` env variable is provided, the following defaults are configured as well:

- `nats_servers`: `"nats://<FLY_REGION>.<ANYCABLE_FLY_WS_APP_NAME>.internal:4222"`
- `http_broadcast_url`: `"http://<FLY_REGION>.<ANYCABLE_FLY_WS_APP_NAME>.internal:8090/_broadcast"`

## gRPC settings

**rpc_host** (`ANYCABLE_RPC_HOST`, `--rpc-host`)

Local address to run gRPC server on (default: `"127.0.0.1:50051"`). Set it to `0.0.0.0:50051` to make gRPC server accessible to the outside world (for example, when using containerized environment).

**rpc_tls_cert** (`ANYCABLE_RPC_TLS_CERT`, `--rpc-tls-cert`) and **rpc_tls_key** (`ANYCABLE_RPC_TLS_KEY`, `--rpc-tls-key`)

Specify file paths or contents for TLS certificate and private key for gRPC server.

### Concurrency settings

AnyCable gRPC server maintains a pool of worker threads to execute commands. We rely on the `grpc` gem [default pool size](https://github.com/grpc/grpc/blob/80e834abab5dff45e16e9a1e3b98f20eae5f91ad/src/ruby/lib/grpc/generic/rpc_server.rb#L163), which is equal to **30**.

You can configure the pool size via `rpc_pool_size` parameter (or `ANYCABLE_RPC_POOL_SIZE` env var).

Increasing pool size makes sense if you have a lot of IO operations in your channels (DB, HTTP, etc.).

**NOTE**: Make sure the gRPC pool size is aligned with concurrency limits you have in your application, such as database pool size.

**IMPORTANT**: AnyCable server concurrency limit must correlate to the RPC server pool size (read more [here](../anycable-go/rpc.md#concurrency-settings)).

### Alternative gRPC implementations

AnyCable Ruby uses the `grpc` gem to run its gRPC server by default. The gem heavily relies on native extensions, which may lead to complications during installation (e.g., on Alpine Linux) and compatibility issues with modern Ruby versions.

To be closer to Ruby and depend less on extensions, AnyCable Ruby also supports an alternative gRPC implementation—[grpc_kit](https://github.com/cookpad/grpc_kit).

You can opt-in to use `grpc_kit` by setting `ANYCABLE_GRPC_IMPL=grpc_kit` environment variable for your `bundle exec anycable` process. You also need to update your `Gemfile` to include the `grpc_kit` gem and gRPC-less versions of AnyCable gems:

```ruby
# For Rails applications
gem "anycable-rails-core", require: ["anycable-rails"]
gem "grpc_kit"

# For non-Rails applications
gem "anycable-core", require: ["anycable"]
gem "grpc_kit"
```
