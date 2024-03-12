# RPC over HTTP

AnyCable supports RPC communication between a real-time server and your web application over HTTP. Although the default gRPC communication is more performant and, thus, preferred, it comes with a price of additional infrastructure complexity—you have to manage a separate process/service.

AnyCable Ruby comes with a Rack middleware that implements AnyCable RPC API. You can mount it into your Rack-compatible application.

> See the [demo](https://github.com/anycable/anycable_rails_demo/pull/1) of using HTTP RPC in a Rails application.

Learn more about different RPC modes in the [AnyCable documentation](../anycable-go/rpc.md).

## Using with Rails

To enable HTTP RPC in your Rails application, all you need is to configure the `http_rpc_mount_path` parameter. For example, in your `config/anycable.yml`:

```yml
development:
  http_rpc_mount_path: "/_anycable"

production:
  http_rpc_mount_path: "/__some_other_anycable_path"
```

That's it! Now configure your AnyCable server to perform RPC over HTTP at your mount path (e.g., `/_anycable`).

**NOTE:** If you don't use AnyCable gRPC server in any environment, you can avoid installing gRPC dependencies by using the `anycable-rails-core` gem instead of `anycable-rails`.

## Security

You can (and MUST in production) protect your HTTP RPC server with basic token-based authorization. To do so, you need to set the `http_rpc_secret` parameter (in YAML or via the `ANYCABLE_HTTP_RPC_SECRET` environment variable). Don't forget to set the same value in your WebSocket server configuration.

## Considerations

- **Performance**. HTTP/1 has a higher overhead than HTTP/2 used by gRPC, so you should expect a higher latency and lower throughput. Keep this in mind when choosing between HTTP RPC and gRPC.

- **Shared web server resources**. Rails applications have a limited HTTP concurrency (based on the total number of threads used by a web server, such as Puma), serving both regular HTTP requests and AnyCable RPC requests can result into a race for shared resources, and, in the worst case, longer request queuing times for user-facing HTTP operations.

- **Scalability**. It's not possible to scale AnyCable RPC requests separately from the main web application. If you need to scale AnyCable RPC requests independently, you should use gRPC.

## Using with Rack

You can mount AnyCable HTTP RPC server into your Rack application using the Rack Builder interface:

```ruby
Rack::Builder.new do
  map "/anycable" do
    run AnyCable::HTTPRC::Server.new
  end
end
```
