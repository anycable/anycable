# RPC Server Middlewares

AnyCable server allows to add custom _middlewares_ (=gRPC interceptors).

For example, `anycable-rails` ships with [the middleware](https://github.com/anycable/anycable-rails/blob/master/lib/anycable/rails/middlewares/executor.rb) that integrate [Rails Executor](https://guides.rubyonrails.org/v5.2.0/threading_and_code_execution.html#framework-behavior) into RPC server.

## Adding custom middleware

AnyCable middleware is a class inherited from `AnyCable::Middleware` and implementing `#call` method:

```ruby
class PrintMiddleware < AnyCable::Middleware
  # request - is a request payload (incoming message)
  # rpc_call - is an active gRPC call
  # handler - is a method (Method object) of RPC handler which is called
  def call(request, rpc_call, handler)
    p request
    yield
  end
end
```

**NOTE**: you MUST yield the execution; it's impossible to halt the execution and respond with data from middleware (you can only raise an exception).

Activate your middleware by adding it to the middleware chain:

```ruby
# anywhere in your app before AnyCable server starts
AnyCable.middleware.use(PrintMiddleware)

# or using instance
AnyCable.middleware.use(ParameterizedMiddleware.new(params))
```
