# frozen_string_literal: true

require "anycable/version"
require "anycable/config"
require "anycable/server"
require "anycable/pubsub"
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
    def logger=(logger)
      @logger = logger
    end

    def logger
      @logger ||= Logger.new(STDOUT).tap do |logger|
        logger.level = Anycable.config.log
      end
    end

    def config
      @config ||= Config.new
    end

    def configure
      yield(config) if block_given?
    end

    def pubsub
      @pubsub ||= PubSub.new
    end

    # Broadcast message to the channel
    def broadcast(channel, payload)
      pubsub.broadcast(channel, payload)
    end
  end
end
