# RPC Server Middlewares

AnyCable server allows to add custom _middlewares_.

For example, `anycable-rails` ships with [the middleware](https://github.com/anycable/anycable-rails/blob/master/lib/anycable/rails/middlewares/executor.rb) that integrate [Rails Executor](https://guides.rubyonrails.org/v5.2.0/threading_and_code_execution.html#framework-behavior) into RPC server.

Exceptions handling is also implemented via [AnyCable middleware](https://github.com/anycable/anycable/blob/master/lib/anycable/middlewares/exceptions.rb).

## Adding custom middleware

AnyCable middleware is a class inherited from `AnyCable::Middleware` and implementing `#call` method:

```ruby
class PrintMiddleware < AnyCable::Middleware
  # request - is a request payload (incoming message)
  # handler - is a method (Symbol) of RPC handler which is called
  def call(handler, request)
    p request
    yield
  end
end
```

**NOTE**: you MUST yield the execution to continue calling middlewares and the RPC handler itself.

Activate your middleware by adding it to the middleware chain:

```ruby
# anywhere in your app before AnyCable server starts
AnyCable.middleware.use(PrintMiddleware)

# or using instance
AnyCable.middleware.use(ParameterizedMiddleware.new(params))
```
