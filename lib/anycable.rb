# frozen_string_literal: true

require "anycable/version"
require "anycable/config"
require "logger"

require "anycable/exceptions_handling"
require "anycable/broadcast_adapters"

require "anycable/middleware_chain"

require "anycable/socket"
require "anycable/rpc"
require "anycable/health_server"

# AnyCable allows to use any websocket service (written in any language) as a replacement
# for ActionCable server.
#
# AnyCable includes an RPC server (gRPC by default), which is used by external WS server to execute commands
# (authentication, subscription authorization, client-to-server messages).
#
# Broadcasting messages to WS is done through _broadcast adapter_ (Redis Pub/Sub by default).
module AnyCable
  class << self
    # Provide connection factory which
    # is a callable object with build
    # a Connection object
    attr_accessor :connection_factory

    # Provide a method to build a server to serve RPC
    attr_accessor :server_builder

    attr_writer :logger

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
    def broadcast(channel, payload)
      broadcast_adapter.broadcast(channel, payload)
    end

    private

    attr_writer :middleware
  end

  self.middleware = MiddlewareChain.new
end

# gRPC is the default for now, so, let's try to load it.
begin
  require "anycable/grpc"
rescue LoadError
end
