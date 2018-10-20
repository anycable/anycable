# frozen_string_literal: true

require 'grpc'
require 'anycable/rpc_handler'
require 'anycable/health_server'

module Anycable
  # Wrapper over gRPC server.
  #
  # Basic example:
  #
  #   # create new server listening on [::]:50051 (default host)
  #   server = Anycable::Server.new(host: "[::]:50051")
  #
  #   # run gRPC server in bakground
  #   server.start
  #
  #   # stop server
  #   server.stop
  class Server
    class << self
      # TODO: deprecate me
      # rubocop:disable Metrics/AbcSize, Metrics/MethodLength
      def start(**options)
        log_grpc! if Anycable.config.log_grpc

        server = new(host: Anycable.config.rpc_host, **Anycable.config.to_grpc_params, **options)

        if Anycable.config.http_health_port_provided?
          health_server = Anycable::HealthServer.new(
            server,
            **Anycable.config.to_http_health_params
          )
          health_server.start
        end

        at_exit do
          server.stop
          health_server&.stop
        end

        Anycable.logger.info "Broadcasting Redis channel: #{Anycable.config.redis_channel}"

        server.start
        server.wait_till_terminated
      end
      # rubocop:enable Metrics/AbcSize, Metrics/MethodLength

      # FIXME: move out of server
      def log_grpc!
        GRPC.define_singleton_method(:logger) { Anycable.logger }
      end
    end

    DEFAULT_HOST = "0.0.0.0:50051"

    attr_reader :grpc_server, :host

    def initialize(host: DEFAULT_HOST, logger: Anycable.logger, **options)
      @logger = logger
      @host = host
      @grpc_server = build_server(options)
    end

    # Start gRPC server in background and
    # wait untill it ready to accept connections
    def start
      return if running?

      raise "Cannot re-start stopped server" if stopped?

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
        server.handle(Anycable::RPCHandler)
      end
    end
  end
end
