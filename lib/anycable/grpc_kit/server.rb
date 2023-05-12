# frozen_string_literal: true

require "anycable/grpc/handler"

require_relative "./health_pb"
require_relative "./health_services_pb"
require_relative "./health_checker"

module AnyCable
  module GRPC
    raise LoadError, "AnyCable::GRPC::Server has been already loaded!" if defined?(AnyCable::GRPC::Server)

    using(Module.new do
      refine ::GrpcKit::Server do
        attr_reader :max_pool_size

        def stopped?
          @stopping
        end
      end
    end)

    # Wrapper over gRPC kit server.
    #
    # Basic example:
    #
    #   # create new server listening on the loopback interface with 50051 port
    #   server = AnyCable::GrpcKit::Server.new(host: "127.0.0.1:50051")
    #
    #   # run gRPC server in bakground
    #   server.start
    #
    #   # stop server
    #   server.stop
    class Server
      attr_reader :grpc_server, :host, :hostname, :port, :sock

      def initialize(host:, logger: nil, **options)
        @logger = logger
        @host = host

        host_parts = host.match(/\A(?<hostname>.+):(?<port>\d{2,5})\z/)

        @hostname = host_parts[:hostname]
        @port = host_parts[:port].to_i

        @grpc_server = build_server(**options)
      end

      # Start gRPC server in background and
      # wait untill it ready to accept connections
      def start
        return if running?

        raise "Cannot re-start stopped server" if stopped?

        logger.info "RPC server (grpc_kit) is starting..."

        @sock = TCPServer.new(hostname, port)

        server = grpc_server

        @start_thread = Thread.new do
          loop do
            conn = @sock.accept
            server.run(conn)
          rescue IOError
            # ignore broken connections
          end
        end

        wait_till_running

        logger.info "RPC server is listening on #{host} (workers_num: #{grpc_server.max_pool_size})"
      end

      def wait_till_running
        raise "Server is not running" unless running?

        timeout = 5

        loop do
          sock = TCPSocket.new(hostname, port, connect_timeout: 1)
          stub = ::Grpc::Health::V1::Health::Stub.new(sock)
          stub.check(::Grpc::Health::V1::HealthCheckRequest.new)
          sock.close
          break
        rescue Errno::ECONNREFUSED, Errno::EHOSTUNREACH, SocketError
          timeout -= 1
          raise "Server is not responding" if timeout.zero?
        end
      end

      def wait_till_terminated
        raise "Server is not running" unless running?

        start_thread.join
      end

      # Stop gRPC server if it's running
      def stop
        return unless running?

        return if stopped?

        grpc_server.graceful_shutdown
        sock.close

        logger.info "RPC server stopped"
      end

      def running?
        !!sock
      end

      def stopped?
        grpc_server.stopped?
      end

      private

      attr_reader :start_thread

      def logger
        @logger ||= AnyCable.logger
      end

      def build_server(**options)
        pool_size = options[:pool_size]

        ::GrpcKit::Server.new(min_pool_size: pool_size, max_pool_size: pool_size).tap do |server|
          server.handle(AnyCable::GRPC::Handler)
          server.handle(build_health_checker)
        end
      end

      def build_health_checker
        health_checker = ::Grpc::Health::Checker.new
        health_checker.add_status(
          "anycable.RPC",
          ::Grpc::Health::V1::HealthCheckResponse::ServingStatus::SERVING
        )
        health_checker.add_status(
          "",
          ::Grpc::Health::V1::HealthCheckResponse::ServingStatus::SERVING
        )
        health_checker
      end
    end
  end
end
