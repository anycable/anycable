# frozen_string_literal: true

require "anycable/version"
require "anycable/config"
require "logger"

require "anycable/exceptions_handling"
require "anycable/broadcast_adapters"

require "anycable/middleware_chain"
require "anycable/middlewares/exceptions"
require "anycable/middlewares/check_version"
require "anycable/middlewares/env_sid"

require "anycable/socket"
require "anycable/rpc"
require "anycable/health_server"

require "anycable/httrpc/server"

# AnyCable allows to use any websocket service (written in any language) as a replacement
# for ActionCable server.
#
# AnyCable includes an RPC server (gRPC by default), which is used by external WS server to execute commands
# (authentication, subscription authorization, client-to-server messages).
#
# Broadcasting messages to WS is done through _broadcast adapter_ (Redis Pub/Sub by default).
module AnyCable
  autoload :Streams, "anycable/streams"

  class << self
    # Provide connection factory which
    # is a callable object with build
    # a Connection object
    attr_accessor :connection_factory

    # Provide a method to build a server to serve RPC
    attr_accessor :server_builder

    attr_writer :logger, :rpc_handler

    attr_reader :middleware

    def logger
      return @logger if instance_variable_defined?(:@logger)

      log_output = AnyCable.config.log_file || $stdout
      @logger = Logger.new(log_output).tap do |logger|
        logger.level = AnyCable.config.log_level
      end
    end

    def config
      @config ||= Config.new
    end

    def configure
      yield(config) if block_given?
    end

    # Register a custom block that will be called
    # when an exception is raised during RPC call
    def capture_exception(&block)
      ExceptionsHandling << block
    end

    # Register a callback to be invoked before
    # the server starts
    def configure_server(&block)
      server_callbacks << block
    end

    def server_callbacks
      @server_callbacks ||= []
    end

    def broadcast_adapter
      self.broadcast_adapter = AnyCable.config.broadcast_adapter.to_sym unless instance_variable_defined?(:@broadcast_adapter)
      @broadcast_adapter
    end

    def broadcast_adapter=(adapter)
      if adapter.is_a?(Symbol) || adapter.is_a?(Array)
        adapter = BroadcastAdapters.lookup_adapter(adapter)
      end

      unless adapter.respond_to?(:broadcast)
        raise ArgumentError, "BroadcastAdapter must implement #broadcast method. " \
                             "#{adapter.class} doesn't implement it."
      end

      @broadcast_adapter = adapter
    end

    # Raw broadcast message to the channel, sends only string!
    # To send hash or object use ActionCable.server.broadcast instead!
    def broadcast(...)
      broadcast_adapter.broadcast(...)
    end

    def rpc_handler
      @rpc_handler ||= AnyCable::RPC::Handler.new
    end

    private

    attr_writer :middleware
  end

  self.middleware = MiddlewareChain.new.tap do |chain|
    # Include exceptions handling middleware by default
    chain.use(Middlewares::Exceptions)
  end
end

# Try loading a gRPC implementation
impl = ENV.fetch("ANYCABLE_GRPC_IMPL", "grpc")

case impl
when "grpc"
  begin
    require "grpc/version"
    require "anycable/grpc"
  rescue LoadError => e
    # Re-raise an exception if we failed to load grpc .so files
    # (e.g., on Alpine Linux)
    raise if /(error loading shared library|incompatible architecture)/i.match?(e.message)
  end
when "grpc_kit"
  begin
    require "grpc_kit/version"
    require "anycable/grpc_kit"
  rescue LoadError
  end
end
