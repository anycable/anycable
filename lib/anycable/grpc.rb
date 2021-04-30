# frozen_string_literal: true

require "grpc"

module AnyCable
  module GRPC
  end
end

require "anycable/grpc/config"
require "anycable/grpc/server"

AnyCable.server_builder = ->(config) {
  AnyCable.logger.info "gRPC version: #{::GRPC::VERSION}"

  ::GRPC.define_singleton_method(:logger) { AnyCable.logger } if config.log_grpc?

  params = config.to_grpc_params
  params[:host] = config.rpc_host

  AnyCable::GRPC::Server.new(**params)
}
