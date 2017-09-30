# frozen_string_literal: true

require "anyway_config"

module Anycable
  # Anycable configuration.
  class Config < Anyway::Config
    config_name :anycable

    attr_config :connection_factory,
                rpc_host: "localhost:50051",
                redis_url: "redis://localhost:6379/5",
                redis_channel: "__anycable__",
                log: :info,
                log_grpc: false,
                debug: false # Shortcut to enable GRPC logging and debug level

    def initialize(*)
      super
      # Set log params if debug is true
      return unless debug
      self.log = :debug
      self.log_grpc = true
    end
  end
end
