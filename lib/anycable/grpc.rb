# frozen_string_literal: true

require "grpc"

module AnyCable
  module GRPC
  end
end

require "anycable/grpc/config"
require "anycable/grpc/server"
require "anycable/grpc/check_version"

AnyCable.server_builder = ->(config) {
  AnyCable.logger.info "gRPC version: #{::GRPC::VERSION}"

  ::GRPC.define_singleton_method(:logger) { AnyCable.logger } if config.log_grpc?

  interceptors = []

  if config.version_check_enabled?
    interceptors << AnyCable::GRPC::CheckVersion.new(AnyCable::PROTO_VERSION)
  end

  params = config.to_grpc_params
  params[:host] = config.rpc_host
  params[:interceptors] = interceptors

  AnyCable::GRPC::Server.new(**params)
}
