# frozen_string_literal: true

require 'grpc'
require 'anycable/rpc_handler'

module Anycable
  # Wrapper over GRPC server
  module Server
    class << self
      attr_reader :grpc_server

      def start
        log_grpc! if Anycable.config.log_grpc
        @grpc_server ||= build_server

        Anycable.logger.info "RPC server is listening on #{Anycable.config.rpc_host}"
        grpc_server.run_till_terminated
      end

      def stop
        return unless running?
        @grpc_server.stop
      end

      def running?
        grpc_server&.running_state == :running
      end

      # Enable GRPC logging
      def log_grpc!
        GRPC.define_singletion_method(:logger) { Anycable.logger }
      end

      private

      def build_server
        GRPC::RpcServer.new.tap do |server|
          server.add_http2_port(Anycable.config.rpc_host, :this_port_is_insecure)
          server.handle(Anycable::RPCHandler)
        end
      end
    end
  end
end
