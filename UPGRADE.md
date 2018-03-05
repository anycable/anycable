# From 0.5.x to 0.6.x

Update your CLI options from:

```
anycable-go --addr=0.0.0.0:8080 --wspath=/cable --rpc=0.0.0.0:50051 --redis=redis://localhost:6379/5 --log
```

to:

```
anycable-go --host=0.0.0.0 --port=8080 --path=/cable --rpc_host=0.0.0.0:50051 --redis_url=redis://localhost:6379/5 --debug
```

Update your env vars according to the table:

0.5.x   | 0.6.x
--------|-------
RPC     |  ANYCABLE_RPC_HOST
REDIS   |  ANYCABLE_REDIS_URL (or REDIS_URL)
REDIS_CHANNEL | ANYCABLE_REDIS_CHANNEL
ADDR     |  Two vars: ANYCABLE_PORT (or PORT) and ANYCABLE_HOST
WSPATH | ANYCABLE_PATH
LOG  | ANYCABLE_DEBUG

Other variables should be prefixed with `ANYCABLE_` (e.g. `ANYCABLE_HEADERS` instead of `HEADERS`).
