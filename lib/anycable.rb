# frozen_string_literal: true

require "anycable/version"
require "anycable/config"
require "logger"

require "anycable/exceptions_handling"
require "anycable/broadcast_adapters"
require "anycable/server"

# Anycable allows to use any websocket service (written in any language) as a replacement
# for ActionCable server.
#
# Anycable includes a gRPC server, which is used by external WS server to execute commands
# (authentication, subscription authorization, client-to-server messages).
#
# Broadcasting messages to WS is done through _broadcast adapter_ (Redis Pub/Sub by default).
module Anycable
  class << self
    # Provide connection factory which
    # is a callable object with build
    # a Connection object
    attr_accessor :connection_factory

    attr_writer :logger

    def logger
      return @logger if instance_variable_defined?(:@logger)

      log_output = Anycable.config.log_file || STDOUT
      @logger = Logger.new(log_output).tap do |logger|
        logger.level = Anycable.config.log_level
      end
    end

    def config
      @config ||= Config.new
    end

    def configure
      yield(config) if block_given?
    end

    # Register a custom block that will be called
    # when an exception is raised during gRPC call
    def capture_exception(&block)
      ExceptionsHandling << block
    end

    def error_handlers
      warn <<~DEPRECATION
        Using `Anycable.error_handlers` is deprecated!
        Please, use `AnyCable.capture_exception` instead.
      DEPRECATION
      ExceptionsHandling
    end

    def broadcast_adapter
      self.broadcast_adapter = :redis unless instance_variable_defined?(:@broadcast_adapter)
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
  end
end

# Add default exceptions handler: print error message to log
Anycable.capture_exception { |e| Anycable.logger.error(e.message) }
