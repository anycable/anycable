# frozen_string_literal: true

require 'grpc'
require 'anycable/rpc_handler'
require 'anycable/health_server'

module Anycable
  # Wrapper over GRPC server
  module Server
    class << self
      attr_reader :grpc_server

      def start
        log_grpc! if Anycable.config.log_grpc

        start_http_health_server
        start_grpc_server
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
        GRPC.define_singleton_method(:logger) { Anycable.logger }
      end

      private

      def start_grpc_server
        @grpc_server ||= build_server

        Anycable.logger.info "RPC server is listening on #{Anycable.config.rpc_host}"
        Anycable.logger.info "Broadcasting Redis channel: #{Anycable.config.redis_channel}"

        grpc_server.run_till_terminated
      end

      def build_server
        GRPC::RpcServer.new(Anycable.config.to_grpc_params).tap do |server|
          server.add_http2_port(Anycable.config.rpc_host, :this_port_is_insecure)
          server.handle(Anycable::RPCHandler)
        end
      end

      def start_http_health_server
        return unless Anycable.config.http_health_port_provided?

        Anycable::HealthServer.start(Anycable.config.http_health_port)
        at_exit { Anycable::HealthServer.stop }
      end
    end
  end
end
