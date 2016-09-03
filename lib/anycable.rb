# frozen_string_literal: true
require "anycable/version"
require "anycable/config"
require "anycable/actioncable/server"

# Anycable allows to use any websocket service (written in any language) as a replacement
# for ActionCable server.
#
# Anycable includes a gRPC server, which is used by external WS server to execute commands
# (authentication, subscription authorization, client-to-server messages).
#
# Broadcasting messages to WS is done through Redis Pub/Sub.
module Anycable
  def self.logger=(logger)
    @logger = logger
  end

  def self.logger
    @logger ||= Anycable.config.debug ? Logger.new(STDOUT) : Logger.new('/dev/null')
  end

  def self.config
    @config ||= Config.new
  end

  def self.configure
    yield(config) if block_given?
  end
end
