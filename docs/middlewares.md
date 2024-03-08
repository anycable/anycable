# RPC middlewares

AnyCable Ruby allows to add custom _middlewares_ to the RPC server (both standalone gRPC and embedded HTTP versions).

For example, `anycable-rails` ships with [the middleware](https://github.com/anycable/anycable-rails/blob/1-4-stable/lib/anycable/rails/middlewares/executor.rb) that integrate [Rails Executor](https://guides.rubyonrails.org/v7.1.0/threading_and_code_execution.html#framework-behavior) into RPC server.

Exceptions handling is also implemented via [AnyCable middleware](https://github.com/anycable/anycable/blob/1-4-stable/lib/anycable/middlewares/exceptions.rb).

## Adding a custom middleware

AnyCable middleware is a class inherited from `AnyCable::Middleware` and implementing `#call` method:

```ruby
class PrintMiddleware < AnyCable::Middleware
  # request - is a request payload (incoming message)
  # handler - is a method (Symbol) of RPC handler which is called
  # meta - is a metadata (Hash) provided along with request
  def call(handler, request, meta)
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
