# frozen_string_literal: true

require "anycable/middleware"
require "monitor"

module AnyCable
  # Middleware chain is used to build the list of
  # gRPC server interceptors.
  #
  # Each interceptor should be a subsclass of
  # AnyCable::Middleware and implement `#call` method.
  class MiddlewareChain
    def initialize
      @registry = []
      @mu = Monitor.new
    end

    def use(middleware)
      check_frozen!
      middleware = build_middleware(middleware)
      sync { registry << middleware }
    end

    def freeze
      registry.freeze
      super
    end

    def to_a
      registry
    end

    private

    attr_reader :mu, :registry

    def sync
      mu.synchronize { yield }
    end

    def check_frozen!
      raise "Cannot modify AnyCable middlewares after server started" if frozen?
    end

    def build_middleware(middleware)
      middleware = middleware.new if
        middleware.is_a?(Class) && middleware <= AnyCable::Middleware

      unless middleware.is_a?(AnyCable::Middleware)
        raise ArgumentError,
              "AnyCable middleware must be a subclass of AnyCable::Middleware, " \
              "got #{middleware} instead"
      end

      middleware
    end
  end
end
