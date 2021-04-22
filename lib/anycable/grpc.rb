# frozen_string_literal: true

require "grpc"

module AnyCable
  module GRPC
  end
end

require "anycable/grpc/config"
require "anycable/grpc/server"

AnyCable.server_builder = proc do |config|
  AnyCable.logger.info "gRPC version: #{::GRPC::VERSION}"

  ::GRPC.define_singleton_method(:logger) { AnyCable.logger } if config.log_grpc?

  AnyCable::GRPC::Server.new(
    host: config.rpc_host,
    **config.to_grpc_params,
    interceptors: AnyCable.middleware.to_a
  )
end
