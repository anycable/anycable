# frozen_string_literal: true

require "anyway_config"
require "grpc"

module Anycable
  # gRPC server configuration.
  class RPCConfig < Anyway::Config
    # TODO: upgrade anyway_config
    env_prefix :anycable_rpc
    config_name :anycable_rpc

    # TODO: add feature to anyway_config
    # config_file :anycable, key: :rpc

    attr_config(
      host: "0.0.0.0:50051",
      # For defaults see https://github.com/grpc/grpc/blob/master/src/ruby/lib/grpc/generic/rpc_server.rb#L162-L170
      pool_size: GRPC::RpcServer::DEFAULT_POOL_SIZE,
      max_waiting_requests: GRPC::RpcServer::DEFAULT_MAX_WAITING_REQUESTS,
      poll_period: GRPC::RpcServer::DEFAULT_POLL_PERIOD,
      pool_keep_alive: GRPC::Pool::DEFAULT_KEEP_ALIVE,
      # TODO: add note about server args with the link
      # to https://github.com/grpc/grpc/blob/f526602bff029b8db50a8d57134d72da33d8a752/include/grpc/impl/codegen/grpc_types.h#L292-L315
      server_args: {}
    )

    def to_grpc_params
      {
        pool_size: pool_size,
        max_waiting_requests: max_waiting_requests,
        poll_period: poll_period,
        pool_keep_alive: pool_keep_alive,
        server_args: server_args
      }
    end
  end
end
