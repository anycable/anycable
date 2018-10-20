# frozen_string_literal: true

require "anycable/version"
require "anycable/config"
require "logger"

# Anycable allows to use any websocket service (written in any language) as a replacement
# for ActionCable server.
#
# Anycable includes a gRPC server, which is used by external WS server to execute commands
# (authentication, subscription authorization, client-to-server messages).
#
# Broadcasting messages to WS is done through Redis Pub/Sub.
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

    def error_handlers
      return @error_handlers if instance_variable_defined?(:@error_handlers)

      @error_handlers = []
    end

    def pubsub
      @pubsub ||= PubSub.new
    end

    # Raw broadcast message to the channel, sends only string!
    # To send hash or object use ActionCable.server.broadcast instead!
    def broadcast(channel, payload)
      pubsub.broadcast(channel, payload)
    end
  end
end

require "anycable/server"
require "anycable/pubsub"
