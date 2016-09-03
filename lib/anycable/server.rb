# frozen_string_literal: true
require 'grpc'
require 'anycable'
require 'anycable/rpc_handler'

# Set GRPC logger
module GRPC
  def self.logger
    Anycable.logger
  end
end

module Anycable
  # Wrapper over GRPC server
  module Server
    class << self
      attr_accessor :grpc_server

      def start
        @grpc_server = GRPC::RpcServer.new
        grpc_server.add_http2_port(Anycable.config.rpc_host, :this_port_is_insecure)
        grpc_server.handle(Anycable::RPCHandler)
        Anycable.logger.info "RPC server is listening on #{Anycable.config.rpc_host}"
        grpc_server.run_till_terminated
      end
    end
  end
end
