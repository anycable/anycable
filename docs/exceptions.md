# Exceptions Handling

AnyCable captures all exceptions during your application code (your _connection_ and _channels_) execution.

The default behaviour is to log the exceptions with `"error"` level.

You can attach your own exceptions handler, for example, to send notifications somewhere (Honeybadger, Sentry, Airbrake, etc.):

```ruby
# with Honeybadger
AnyCable.capture_exception do |ex, method, message|
  Honeybadger.notify(ex, component: "any_cable", action: method, params: message)
end

# with Raven (Sentry)
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
```
