# AnyCable-Go Health Checking

Health check endpoint is enabled by default and accessible at `/health` path.

You can configure the path via the `--health-path` option (or `ANYCABLE_HEALTH_PATH` env var).

You can use this endpoint as readiness/liveness check (e.g. for load balancers).
