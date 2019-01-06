# frozen_string_literal: true

require "webrick"

module AnyCable
  # Server for HTTP healthchecks.
  #
  # Basic usage:
  #
  #  # create a new healthcheck server for a specified
  #  # gRPC server lisening on the port
  #  health_server = AnyCable::HealthServer.new(grpc_server, port)
  #
  #  # start health server in background
  #  health_server.start
  #
  #  # stop health server
  #  health_server.stop
  class HealthServer
    SUCCESS_RESPONSE = [200, "Ready"].freeze
    FAILURE_RESPONSE = [503, "Not Ready"].freeze

    attr_reader :grpc_server, :port, :path, :server

    def initialize(grpc_server, port:, path: "/health", logger: AnyCable.logger)
      @grpc_server = grpc_server
      @port = port
      @path = path
      @logger = logger
      @server = build_server
    end

    def start
      return if running?

      Thread.new { server.start }

      logger.info "HTTP health server is listening on localhost:#{port} and mounted at \"#{path}\""
    end

    def stop
      return unless running?

      server.shutdown
    end

    def running?
      server.status == :Running
    end

    private

    attr_reader :logger

    def build_server
      WEBrick::HTTPServer.new(
        Port: port,
        Logger: logger,
        AccessLog: []
      ).tap do |server|
        server.mount_proc path do |_, res|
          res.status, res.body = grpc_server.running? ? SUCCESS_RESPONSE : FAILURE_RESPONSE
        end
      end
    end
  end
end
