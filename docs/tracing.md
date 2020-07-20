# AnyCable-Go Tracing

AnyCable-Go assigns a random unique `sid` (_session ID_) or use the one provided in the `X-Request-ID` HTTP header
to each websocket connection and passes it with requests to RPC service. This identifier is also
available in logs and you can use it to trace a request's pathway through the whole Load Balancer -> WS Server -> RPC stack.

Logs example:

```sh
D 2019-04-25T18:41:07.172Z context=node sid=FQQS_IltswlTJK60ncf9Cm Incoming message: &{subscribe {"channel":"PresenceChannel"} }
D 2019-04-25T18:41:08.074Z context=pubsub Incoming pubsub message from Redis: {"stream":"presence:Z2lkOi8vbWFuYWdlYmFjL1NjaG9vbC8xMDAwMjI3Mw","data":"{\"type\":\"presence\",\"event\":\"user-presence-changed\",\"user_id\":1,\"status\":\"online\"}"}
```

## Using with Heroku

Heroku assigns `X-Request-ID` [automatically at the router level](https://devcenter.heroku.com/articles/http-request-id).

## Using with NGINX

If you use AnyCable-Go behind NGINX server, you can assign request id with the provided configuration example:

```nginx
# Сonfiguration is shortened for the sake of brevity

log_format trace '$remote_addr - $remote_user [$time_local] "$request" '
                 '$status $body_bytes_sent "$http_referer" "$http_user_agent" '
                 '"$http_x_forwarded_for" $request_id'; # `trace` logger

server {
    add_header X-Request-ID $request_id; # Return `X-Request-ID` to client

    location /cable {
        proxy_set_header X-Request-ID $request_id; # Pass X-Request-ID` to AnyCable-GO server
        access_log /var/log/nginx/access_trace.log trace; # Use `trace` log
    }
}
```
