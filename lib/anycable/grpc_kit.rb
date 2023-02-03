# frozen_string_literal: true

require "grpc_kit"

module AnyCable
  module GRPC
  end
end

require "anycable/grpc/config"
require "anycable/grpc_kit/server"

AnyCable.server_builder = ->(config) {
  AnyCable.logger.info "gRPC Kit version: #{::GrpcKit::VERSION}"

  ::GrpcKit.loglevel = :fatal
  ::GrpcKit.logger = AnyCable.logger if config.log_grpc?

  params = config.to_grpc_params

  AnyCable::GRPC::Server.new(**params, host: config.rpc_host)
}
