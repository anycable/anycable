# Using AnyCable-Go with Envoy

[Envoy](https://www.envoyproxy.io) is a modern proxy service which support HTTP2 and gRPC.

We can use Envoy for load balancing and zero-disconnect deployments.

## Running an example

Launch 2 RPC servers:

```
# first
$ bundle exec anycable --rpc-host="0.0.0.0:50060"

# second
$ bundle exec anycable --rpc-host="0.0.0.0:50061"
```

Build and run Envoy Docker image:

```
docker rmi -f envoy:v1

docker run -p 9901:9901 -p 50051:50051 --name envoy-cable envoy:v1
```

Now you can access AnyCable RPC service at `:50051`.

Try to restart Ruby processes one by one and see how this affects WebSocket connections (spoiler: they stay connected, no RPC errors in `anycable-go`).
