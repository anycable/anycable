# frozen_string_literal: true

AnyCable::Config.attr_config(
  ### gRPC options
  rpc_host: "127.0.0.1:50051",
  # For defaults see https://github.com/grpc/grpc/blob/51f0d35509bcdaba572d422c4f856208162022de/src/ruby/lib/grpc/generic/rpc_server.rb#L186-L216
  rpc_pool_size: ::GRPC::RpcServer::DEFAULT_POOL_SIZE,
  rpc_max_waiting_requests: ::GRPC::RpcServer::DEFAULT_MAX_WAITING_REQUESTS,
  rpc_poll_period: ::GRPC::RpcServer::DEFAULT_POLL_PERIOD,
  rpc_pool_keep_alive: ::GRPC::Pool::DEFAULT_KEEP_ALIVE,
  # See https://github.com/grpc/grpc/blob/f526602bff029b8db50a8d57134d72da33d8a752/include/grpc/impl/codegen/grpc_types.h#L292-L315
  rpc_server_args: {},
  log_grpc: false
)

AnyCable::Config.ignore_options :rpc_server_args
AnyCable::Config.flag_options :log_grpc

module AnyCable
  module GRPC
    module Config
      def log_grpc
        debug? || super
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
    end
  end
end

AnyCable::Config.prepend AnyCable::GRPC::Config

AnyCable::Config.usage <<~TXT
  GRPC OPTIONS
      --rpc-host=host                   Local address to run gRPC server on, default: "127.0.0.1:50051"
      --rpc-pool-size=size              gRPC workers pool size, default: 30
      --rpc-max-waiting-requests=num    Max waiting requests queue size, default: 20
      --rpc-poll-period=seconds         Poll period (sec), default: 1
      --rpc-pool-keep-alive=seconds     Keep-alive polling interval (sec), default: 1
      --log-grpc                        Enable gRPC logging (disabled by default)
TXT
