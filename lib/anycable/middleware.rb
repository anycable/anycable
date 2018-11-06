# frozen_string_literal: true

require "grpc"

module AnyCable
  # Middleware is a wrapper over gRPC interceptors
  # for request/response calls
  class Middleware < GRPC::Interceptor
    def request_response(request: nil, call: nil, method: nil)
      call(request, call, method) do
        yield
      end
    end

    def call(*)
      raise NotImplementedError
    end
  end
end
