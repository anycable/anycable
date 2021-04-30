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

    def call(method_name, request, meta = {}, &block)
      return yield(method_name, request, meta) if registry.none?

      execute_next_middleware(0, method_name, request, meta, block)
    end

    private

    def execute_next_middleware(ind, method_name, request, meta, block)
      return block.call(method_name, request, meta) if ind >= registry.size

      registry[ind].call(method_name, request, meta) do
        execute_next_middleware(ind + 1, method_name, request, meta, block)
      end
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
