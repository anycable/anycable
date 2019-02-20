# frozen_string_literal: true

require "anyway_config"
require "grpc"

module AnyCable
  # AnyCable configuration.
  class Config < Anyway::Config
    config_name :anycable

    DefaultHostWrapper = Class.new(String)

    attr_config(
      ### gRPC options
      rpc_host: DefaultHostWrapper.new("[::]:50051"),
      # For defaults see https://github.com/grpc/grpc/blob/51f0d35509bcdaba572d422c4f856208162022de/src/ruby/lib/grpc/generic/rpc_server.rb#L186-L216
      rpc_pool_size: GRPC::RpcServer::DEFAULT_POOL_SIZE,
      rpc_max_waiting_requests: GRPC::RpcServer::DEFAULT_MAX_WAITING_REQUESTS,
      rpc_poll_period: GRPC::RpcServer::DEFAULT_POLL_PERIOD,
      rpc_pool_keep_alive: GRPC::Pool::DEFAULT_KEEP_ALIVE,
      # See https://github.com/grpc/grpc/blob/f526602bff029b8db50a8d57134d72da33d8a752/include/grpc/impl/codegen/grpc_types.h#L292-L315
      rpc_server_args: {},

      ### Redis options
      redis_url: ENV.fetch("REDIS_URL", "redis://localhost:6379/5"),
      redis_sentinels: nil,
      redis_channel: "__anycable__",

      ### Logging options
      log_file: nil,
      log_level: :info,
      log_grpc: false,
      debug: false, # Shortcut to enable GRPC logging and debug level

      ### Health check options
      http_health_port: nil,
      http_health_path: "/health"
    )

    ignore_options :rpc_server_args
    flag_options :log_grpc, :debug

    def log_level
      debug ? :debug : @log_level
    end

    def log_grpc
      debug || @log_grpc
    end

    def debug
      @debug != false
    end

    alias debug? debug

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
        next if redis_sentinels.nil?

        raise ArgumentError, "redis_sentinels must be an array; got #{redis_sentinels}" unless
          redis_sentinels.is_a?(Array)

        next if redis_sentinels.empty?

        params[:sentinels] = redis_sentinels.map(&method(:parse_sentinel))
      end
    end

    # Build HTTP health server parameters
    def to_http_health_params
      {
        port: http_health_port,
        path: http_health_path
      }
    end

    private

    SENTINEL_RXP = /^([\w\-_]*)\:(\d+)$/.freeze

    def parse_sentinel(sentinel)
      return sentinel if sentinel.is_a?(Hash)

      matches = sentinel.match(SENTINEL_RXP)

      raise ArgumentError, "Invalid Sentinel value: #{sentinel}" if matches.nil?

      { "host" => matches[1], "port" => matches[2].to_i }
    end
  end
end
