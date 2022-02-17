# frozen_string_literal: true

module AnyCable
  # Server for HTTP healthchecks.
  #
  # Basic usage:
  #
  #  # create a new healthcheck server for a specified
  #  # server listening on the port
  #  health_server = AnyCable::HealthServer.new(server, port)
  #
  #  # start health server in background
  #  health_server.start
  #
  #  # stop health server
  #  health_server.stop
  class HealthServer
    SUCCESS_RESPONSE = [200, "Ready"].freeze
    FAILURE_RESPONSE = [503, "Not Ready"].freeze

    attr_reader :server, :port, :path, :http_server

    def initialize(server, port:, logger: nil, path: "/health")
      @server = server
      @port = port
      @path = path
      @logger = logger
      @http_server = build_server
    end

    def start
      return if running?

      Thread.new { http_server.start }

      logger.info "HTTP health server is listening on localhost:#{port} and mounted at \"#{path}\""
    end

    def stop
      return unless running?

      http_server.shutdown
    end

    def running?
      http_server.status == :Running
    end

    private

    def logger
      @logger ||= AnyCable.logger
    end

    def build_server
      begin
        require "webrick"
      rescue LoadError
        raise "Please, install webrick gem to use health server"
      end

      WEBrick::HTTPServer.new(
        Port: port,
        Logger: logger,
        AccessLog: []
      ).tap do |http_server|
        http_server.mount_proc path do |_, res|
          # Replace with mass assignment as soon as Steep added support
          # https://github.com/soutaro/steep/issues/424
          if server.running?
            res.status = SUCCESS_RESPONSE.first
            res.body = SUCCESS_RESPONSE.last
          else
            res.status = FAILURE_RESPONSE.first
            res.body = FAILURE_RESPONSE.last
          end
        end
      end
    end
  end
end
