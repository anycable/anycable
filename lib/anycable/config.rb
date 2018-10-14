# frozen_string_literal: true

require "anyway_config"
require "grpc"

module Anycable
  # Anycable configuration.
  class Config < Anyway::Config
    config_name :anycable

    attr_config(
      ### gRPC options
      rpc_host: "0.0.0.0:50051",
      # For defaults see https://github.com/grpc/grpc/blob/master/src/ruby/lib/grpc/generic/rpc_server.rb#L162-L170
      rpc_pool_size: GRPC::RpcServer::DEFAULT_POOL_SIZE,
      rpc_max_waiting_requests: GRPC::RpcServer::DEFAULT_MAX_WAITING_REQUESTS,
      rpc_poll_period: GRPC::RpcServer::DEFAULT_POLL_PERIOD,
      rpc_pool_keep_alive: GRPC::Pool::DEFAULT_KEEP_ALIVE,
      # See https://github.com/grpc/grpc/blob/f526602bff029b8db50a8d57134d72da33d8a752/include/grpc/impl/codegen/grpc_types.h#L292-L315
      rpc_server_args: {},

      ### Redis options
      redis_url: ENV.fetch("REDIS_URL", "redis://localhost:6379/5"),
      redis_sentinels: [],
      redis_channel: "__anycable__",

      ### Logging options
      log_file: nil,
      log_level: :info,
      log_grpc: false,
      debug: false, # Shortcut to enable GRPC logging and debug level

      ### Health check options
      http_health_port: nil
    )

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

    # Build gRPC server parameters
    def to_grpc_params
      {
        pool_size: rpc_pool_size,
        max_waiting_requests: rpc_max_waiting_requests,
        poll_period: rpc_poll_period,
        pool_keep_alive: rpc_pool_keep_alive,
        server_args: rpc_server_args
      }
    end

    # Build Redis parameters
    def to_redis_params
      { url: redis_url }.tap do |params|
        params[:sentinels] = redis_sentinels unless redis_sentinels.empty?
      end
    end
  end
end
