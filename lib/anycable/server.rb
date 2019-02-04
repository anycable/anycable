# frozen_string_literal: true

require "grpc"
require "grpc/health/checker"
require "grpc/health/v1/health_services_pb"

require "anycable/rpc_handler"
require "anycable/health_server"

module AnyCable
  # Wrapper over gRPC server.
  #
  # Basic example:
  #
  #   # create new server listening on the loopback interface with 50051 port
  #   server = AnyCable::Server.new(host: "127.0.0.1:50051")
  #
  #   # run gRPC server in bakground
  #   server.start
  #
  #   # stop server
  #   server.stop
  class Server
    class << self
      # rubocop:disable Metrics/AbcSize, Metrics/MethodLength
      def start(**options)
        warn <<~DEPRECATION
          DEPRECATION WARNING: Using AnyCable::Server.start is deprecated!
          Please, use anycable CLI instead.

          See https://docs.anycable.io/#upgrade_to_0_6_0
        DEPRECATION

        AnyCable.server_callbacks.each(&:call)

        server = new(
          host: AnyCable.config.rpc_host,
          **AnyCable.config.to_grpc_params,
          interceptors: AnyCable.middleware.to_a,
          **options
        )

        AnyCable.middleware.freeze

        if AnyCable.config.http_health_port_provided?
          health_server = AnyCable::HealthServer.new(
            server,
            **AnyCable.config.to_http_health_params
          )
          health_server.start
        end

        at_exit do
          server.stop
          health_server&.stop
        end

        AnyCable.logger.info "Broadcasting Redis channel: #{AnyCable.config.redis_channel}"

        server.start
        server.wait_till_terminated
      end
      # rubocop:enable Metrics/AbcSize, Metrics/MethodLength
    end

    attr_reader :grpc_server, :host

    def initialize(host:, logger: AnyCable.logger, **options)
      @logger = logger
      @host = host
      @grpc_server = build_server(options)
    end

    # Start gRPC server in background and
    # wait untill it ready to accept connections
    def start
      return if running?

      raise "Cannot re-start stopped server" if stopped?

      check_default_host

      logger.info "RPC server is starting..."

      @start_thread = Thread.new { grpc_server.run }

      grpc_server.wait_till_running

      logger.info "RPC server is listening on #{host}"
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

    attr_reader :logger, :start_thread

    def build_server(options)
      GRPC::RpcServer.new(options).tap do |server|
        server.add_http2_port(host, :this_port_is_insecure)
        server.handle(AnyCable::RPCHandler)
        server.handle(build_health_checker)
      end
    end

    def build_health_checker
      health_checker = Grpc::Health::Checker.new
      health_checker.add_status(
        "anycable.RPC",
        Grpc::Health::V1::HealthCheckResponse::ServingStatus::SERVING
      )
      health_checker
    end

    def check_default_host
      return unless host.is_a?(Anycable::Config::DefaultHostWrapper)

      warn <<~DEPRECATION
        DEPRECATION WARNING: You're using default rpc_host configuration which starts AnyCable RPC
        server on all available interfaces including external IPv4 and IPv6.
        This is about to be changed to loopback interface only in future versions.

        Please, consider switching to the loopback interface or set "[::]:50051"
        explicitly in your configuration, if you want to continue with the current
        behavior and supress this message.

        See https://docs.anycable.io/#/configuration
      DEPRECATION
    end
  end
end
