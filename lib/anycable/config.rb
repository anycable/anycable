# frozen_string_literal: true

require "anyway_config"

module Anycable
  # Anycable configuration.
  class Config < Anyway::Config
    config_name :anycable

    attr_config rpc_host: "localhost:50051",
                redis_url: "redis://localhost:6379/5",
                redis_sentinels: [],
                redis_channel: "__anycable__",
                log_file: nil,
                log_level: :info,
                log_grpc: false,
                debug: false, # Shortcut to enable GRPC logging and debug level
                http_health_port: nil

    def initialize(*)
      super
      # Set log params if debug is true
      return unless debug

      self.log_level = :debug
      self.log_grpc = true
    end

    def http_health_port_provided?
      !http_health_port.nil? && http_health_port != ""
    end
  end
end
