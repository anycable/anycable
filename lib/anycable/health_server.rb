# frozen_string_literal: true

require 'webrick'
require 'anycable/server'

module Anycable
  # Server for HTTP healthchecks
  module HealthServer
    class << self
      def start(grpc_server, port)
        return unless Anycable.config.http_health_port_provided?
        return if running?

        @health_server ||= build_server(grpc_server, port)
        Thread.new { @health_server.start }
        at_exit { Anycable::HealthServer.stop }

        Anycable.logger.info "HTTP health server is listening on #{port}"
      end

      def stop
        return unless running?

        @health_server.shutdown
      end

      def running?
        @health_server&.status == :Running
      end

      private

      SUCCESS_RESPONSE = [200, "Ready"].freeze
      FAILURE_RESPONSE = [503, "Not Ready"].freeze

      def build_server(grpc_server, port)
        WEBrick::HTTPServer.new(
          Port: port,
          Logger: Anycable.logger,
          AccessLog: []
        ).tap do |server|
          server.mount_proc '/health' do |_, res|
            res.status, res.body = grpc_server.running? ? SUCCESS_RESPONSE : FAILURE_RESPONSE
          end
        end
      end
    end
  end
end
