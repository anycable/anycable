# Exceptions handling

AnyCable Ruby RPC server (both gRPC and HTTP) captures all exceptions during your application code (your _connection_ and _channels_) execution.

The default behaviour is to log the exceptions with `"error"` level.

> AnyCable Rails automatically integrates with Rails 7+ error reporting interface (`Rails.error.report(...)`), so you don't need to configure anything yourself.

You can attach your own exceptions handler, for example, to send notifications somewhere (Honeybadger, Sentry, Airbrake, etc.):

```ruby
# with Honeybadger
AnyCable.capture_exception do |ex, method, message|
  Honeybadger.notify(ex, component: "any_cable", action: method, params: message)
end

# with Sentry (new SDK)...
AnyCable.capture_exception do |ex, method, message|
  Sentry.with_scope do |scope|
    scope.set_tags transaction: "AnyCable#{method}", extra: message
    Sentry.capture_exception(ex)
  end
end

# ...or Raven (legacy Sentry SDK)
AnyCable.capture_exception do |ex, method, message|
  Raven.capture_exception(ex, transaction: "AnyCable##{method}", extra: message)
end

# with Airbrake
AnyCable.capture_exception do |ex, method, message|
  Airbrake.notify(ex) do |notice|
    notice[:context][:component] = "any_cable"
    notice[:context][:action] = method
    notice[:params] = message
  end
end

# with Datadog
AnyCable.capture_exception do |ex, method, message|
  Datadog.tracer.trace("any_cable") do |span|
    span.set_error(ex)
    span.set_tag("method", method)
    span.set_tag("message", message)
  ensure
    span.finish
  end
end
```
