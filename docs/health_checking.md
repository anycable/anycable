# Health Checking

AnyCable provides two types of health checks: HTTP and gRPC.

## HTTP

You can run a health check server along with the RPC server by specifying the `http_health_port`:

```sh
# via CLI options
$ bundle exec anycable --http-health-port=54321
#> ...
#> HTTP health server is listening on localhost:54321 and mounted at "/health"

# or via env
$ ANYCABLE_HTTP_HEALTH_PORT=54321 bundle exec anycable
```

You can also specify the mount path:

```sh
$ bundle exec anycable --http-health-port=54321 --http-health-path="/check"
#> ...
#> HTTP health server is listening on localhost:54321 and mounted at "/check"
```

The health check server responds with 200 when the gRPC server is running and with 503 when it isn't.

HTTP health check server can be used for readiness and liveness checks (e.g., in Kubernetes environment).

## gRPC

AnyCable includes a standard gRPC health checker (v1). See official [documentation](https://github.com/grpc/grpc/blob/master/doc/health-checking.md).
