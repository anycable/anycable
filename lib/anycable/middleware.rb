# frozen_string_literal: true

require "grpc"

module AnyCable
  # Middleware is a wrapper over gRPC interceptors
  # for request/response calls
  class Middleware < GRPC::ServerInterceptor
    def request_response(request: nil, call: nil, method: nil)
      # Call middlewares only for AnyCable service
      return yield unless method.receiver.is_a?(AnyCable::RPCHandler)

      call(request, call, method) do
        yield
      end
    end

    def server_streamer(**kwargs)
      p kwargs
      yield
    end

    def call(*)
      raise NotImplementedError
    end
  end
end
