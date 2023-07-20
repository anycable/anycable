# frozen_string_literal: true

require "grpc"
require "grpc/health/checker"
require "grpc/health/v1/health_services_pb"

require "anycable/grpc/handler"

module AnyCable
  module GRPC
    using(Module.new do
      refine ::GRPC::RpcServer do
        attr_reader :pool_size
      end
    end)

    # Wrapper over gRPC server.
    #
    # Basic example:
    #
    #   # create new server listening on the loopback interface with 50051 port
    #   server = AnyCable::GRPC::Server.new(host: "127.0.0.1:50051")
    #
    #   # run gRPC server in bakground
    #   server.start
    #
    #   # stop server
    #   server.stop
    class Server
      attr_reader :grpc_server, :host

      def initialize(host:, logger: nil, **options)
        @logger = logger
        @host = host
        @grpc_server = build_server(**options)
      end

      # Start gRPC server in background and
      # wait untill it ready to accept connections
      def start
        return if running?

        raise "Cannot re-start stopped server" if stopped?

        logger.info "RPC server is starting..."

        @start_thread = Thread.new { grpc_server.run }

        grpc_server.wait_till_running

        logger.info "RPC server is listening on #{host} (workers_num: #{grpc_server.pool_size})"
      end

      def wait_till_terminated
        raise "Server is not running" unless running?

        start_thread.join
      end

      # Stop gRPC server if it's running
      def stop
        return unless running?

        grpc_server.stop

        logger.info "RPC server stopped"
      end

      def running?
        grpc_server.running_state == :running
      end

      def stopped?
        grpc_server.running_state == :stopped
      end

      private

      attr_reader :start_thread

      def logger
        @logger ||= AnyCable.logger
      end

      def build_server(**options)
        server_credentials = options.delete(:server_credentials)
        ::GRPC::RpcServer.new(**options).tap do |server|
          server.add_http2_port(host, server_credentials)
          server.handle(AnyCable::GRPC::Handler)
          server.handle(build_health_checker)
        end
      end

      def build_health_checker
        health_checker = Grpc::Health::Checker.new
        health_checker.add_status(
          "anycable.RPC",
          Grpc::Health::V1::HealthCheckResponse::ServingStatus::SERVING
        )
        health_checker.add_status(
          "",
          Grpc::Health::V1::HealthCheckResponse::ServingStatus::SERVING
        )
        health_checker
      end
    end
  end
end
