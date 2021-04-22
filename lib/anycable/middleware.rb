# frozen_string_literal: true

module AnyCable
  # Middleware is an analague of Rack middlewares but for AnyCable RPC calls
  class Middleware
    def call(_method_name, _request)
      raise NotImplementedError
    end
  end
end
