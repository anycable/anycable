# Logging

By default, AnyCable logger logs to STDOUT with `INFO` severity but can be easily configured (see [Configuration](configuration.md#parameters)), for example:

```sh
$ bundle exec anycable --log-file=logs/anycable.log --log-level debug

# or

$ ANYCABLE_LOG_FILE=logs/anycable.log ANYCABLE_LOG_LEVEL=debug bundle exec anycable
```

You can also specify your own logger instance for full control:

```ruby
# AnyCable invokes this code before initializing the configuration
AnyCable.logger = MyLogger.new
```

If you use AnyCable with Rails see the corresponding section in [Using with Rails](../rails/getting_started.md#logging)

## gRPC logging

AnyCable does not log any GRPC internal events by default. You can turn GRPC logger on by setting `log_grpc` parameter to true:

```sh
$ bundle exec anycable --log-grpc

# or

$ ANYCABLE_LOG_GRPC=t bundle exec anycable
```

## Debug mode

You can turn on verbose logging (with gRPC logging turned on and log level set to `"debug"`) by using a shortcut parameter–`debug`:

```sh
$ bundle exec anycable --debug

# or

$ ANYCABLE_DEBUG=1 bundle exec anycable
```

## Log tracing

When using with Rails, AnyCable adds a _session ID_ tag (`sid`) to each log entry produced during the RPC message handling. You can use it to trace the request's pathway through the whole Load Balancer -> WS Server -> RPC stack.

Logs example:

```sh
[AnyCable sid=FQQS_IltswlTJK60ncf9Cm] RPC Command: <AnyCable::CommandMessage: command: "subscribe", identifier: "{\"channel\":\"PresenceChannel\"}", connection_identifiers: "{\"current_user\":\"Z2lkOi8vbWFuYWdlYmFjL1VzZXIvMTEwODQ0OTc\"}", data: "", env: <>>
[AnyCable sid=FQQS_IltswlTJK60ncf9Cm]   User Load (0.6ms)  SELECT  `users`.* FROM `users` WHERE `users`.`id` = 1 LIMIT 1
```
