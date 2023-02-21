# Using AnyCable-Go with Envoy

**NOTE:** The example configuration works with v1.16, but no longer valid for the latest versions of Envoy. PRs are welcomed!

[Envoy](https://www.envoyproxy.io) is a modern proxy service which support HTTP2 and gRPC.

We can use Envoy for load balancing and zero-disconnect deployments.

## Running an example

Launch 2 RPC servers:

```sh
# first
bundle exec anycable --rpc-host="0.0.0.0:50060"

# second
bundle exec anycable --rpc-host="0.0.0.0:50061"
```

Run Envoy via the Docker image (from the current directory):

```sh
docker run --rm -p 50051:50051 -v $(pwd):/etc/envoy envoyproxy/envoy:v1.16.1
```

Now you can access AnyCable RPC service at `:50051`.

Try to restart Ruby processes one by one and see how this affects WebSocket connections (spoiler: they stay connected, no RPC errors in `anycable-go`).
